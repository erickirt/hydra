// Copyright © 2022 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package testhelpers

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	"github.com/go-jose/go-jose/v3"
	"github.com/gofrs/uuid"
	"github.com/stretchr/testify/require"

	"github.com/ory/fosite"
	"github.com/ory/fosite/handler/openid"
	"github.com/ory/hydra/v2/client"
	"github.com/ory/hydra/v2/consent"
	"github.com/ory/hydra/v2/driver"
	"github.com/ory/hydra/v2/driver/config"
	"github.com/ory/hydra/v2/flow"
	"github.com/ory/hydra/v2/oauth2"
	"github.com/ory/hydra/v2/oauth2/trust"
	"github.com/ory/hydra/v2/x"
	"github.com/ory/x/configx"
	"github.com/ory/x/contextx"
	"github.com/ory/x/logrusx"

	"github.com/ory/x/sqlxx"
)

type JanitorConsentTestHelper struct {
	uniqueName           string
	flushLoginRequests   []*flow.LoginRequest
	flushConsentRequests []*flow.OAuth2ConsentRequest
	flushAccessRequests  []*fosite.Request
	flushRefreshRequests []*fosite.AccessRequest
	flushGrants          []*createGrantRequest
	conf                 *config.DefaultProvider
}

type createGrantRequest struct {
	grant trust.Grant
	pk    jose.JSONWebKey
}

const lifespan = time.Hour

func NewConsentJanitorTestHelper(t *testing.T, uniqueName string, opts ...configx.OptionModifier) *JanitorConsentTestHelper {
	conf := NewConfigurationWithDefaults(t, append([]configx.OptionModifier{configx.WithValues(map[string]any{
		config.KeyScopeStrategy:        "DEPRECATED_HIERARCHICAL_SCOPE_STRATEGY",
		config.KeyIssuerURL:            "http://hydra.localhost",
		config.KeyAccessTokenLifespan:  lifespan,
		config.KeyRefreshTokenLifespan: lifespan,
		config.KeyConsentRequestMaxAge: lifespan,
		config.KeyLogLevel:             "trace",
	})}, opts...)...)

	return &JanitorConsentTestHelper{
		uniqueName:           uniqueName,
		conf:                 conf,
		flushLoginRequests:   genLoginRequests(uniqueName, lifespan),
		flushConsentRequests: genConsentRequests(uniqueName, lifespan),
		flushAccessRequests:  getAccessRequests(uniqueName, lifespan),
		flushRefreshRequests: getRefreshRequests(uniqueName, lifespan),
		flushGrants:          getGrantRequests(uniqueName, lifespan),
	}
}

func (j *JanitorConsentTestHelper) GetDSN() string {
	return j.conf.DSN()
}

func (j *JanitorConsentTestHelper) GetConfig() *config.DefaultProvider {
	return j.conf
}

var NotAfterTestCycles = map[string]time.Duration{
	"notAfter24h":   lifespan * 24,
	"notAfter1h30m": lifespan + time.Hour/2,
	"notAfterNow":   0,
}

func (j *JanitorConsentTestHelper) GetNotAfterTestCycles() map[string]time.Duration {
	return map[string]time.Duration{}
}

func (j *JanitorConsentTestHelper) GetRegistry(ctx context.Context, dbname string) (driver.Registry, error) {
	j.conf.MustSet(ctx, config.KeyDSN, fmt.Sprintf("sqlite://file:%s?mode=memory&_fk=true&cache=shared", dbname))
	return driver.NewRegistryFromDSN(ctx, j.conf, logrusx.New("test_hydra", "master"), false, true, &contextx.Default{})
}

func (j *JanitorConsentTestHelper) AccessTokenNotAfterSetup(ctx context.Context, cl client.Manager, store x.FositeStorer) func(t *testing.T) {
	return func(t *testing.T) {
		// Create access token clients and session
		for _, r := range j.flushAccessRequests {
			require.NoError(t, cl.CreateClient(ctx, r.Client.(*client.Client)))
			require.NoError(t, store.CreateAccessTokenSession(ctx, r.ID, r))
		}

	}
}

func (j *JanitorConsentTestHelper) AccessTokenNotAfterValidate(ctx context.Context, notAfter time.Time, store x.FositeStorer) func(t *testing.T) {
	return func(t *testing.T) {
		var err error
		ds := new(oauth2.Session)

		accessTokenLifespan := time.Now().Round(time.Second).Add(-j.conf.GetAccessTokenLifespan(ctx))

		for _, r := range j.flushAccessRequests {
			t.Logf("access flush check: %s", r.ID)
			_, err = store.GetAccessTokenSession(ctx, r.ID, ds)
			if j.notAfterCheck(notAfter, accessTokenLifespan, r.RequestedAt) {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		}
	}
}

func (j *JanitorConsentTestHelper) RefreshTokenNotAfterSetup(ctx context.Context, cl client.Manager, store x.FositeStorer) func(t *testing.T) {
	return func(t *testing.T) {
		// Create refresh token clients and session
		for _, fr := range j.flushRefreshRequests {
			require.NoError(t, cl.CreateClient(ctx, fr.Client.(*client.Client)))
			require.NoError(t, store.CreateRefreshTokenSession(ctx, fr.ID, "", fr))
		}
	}
}

func (j *JanitorConsentTestHelper) RefreshTokenNotAfterValidate(ctx context.Context, notAfter time.Time, store x.FositeStorer) func(t *testing.T) {
	return func(t *testing.T) {
		var err error
		ds := new(oauth2.Session)

		refreshTokenLifespan := time.Now().Round(time.Second).Add(-j.conf.GetRefreshTokenLifespan(ctx))

		for _, r := range j.flushRefreshRequests {
			t.Logf("refresh flush check: %s", r.ID)
			_, err = store.GetRefreshTokenSession(ctx, r.ID, ds)
			if j.notAfterCheck(notAfter, refreshTokenLifespan, r.RequestedAt) {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		}
	}
}

func (j *JanitorConsentTestHelper) GrantNotAfterSetup(ctx context.Context, gr trust.GrantManager) func(t *testing.T) {
	return func(t *testing.T) {
		for _, fg := range j.flushGrants {
			require.NoError(t, gr.CreateGrant(ctx, fg.grant, fg.pk))
		}
	}
}

func (j *JanitorConsentTestHelper) GrantNotAfterValidate(ctx context.Context, notAfter time.Time, gr trust.GrantManager) func(t *testing.T) {
	return func(t *testing.T) {
		var err error

		// flush won't delete grants that have not yet expired, so use now to check that
		deleteUntil := time.Now().Round(time.Second)
		if deleteUntil.After(notAfter) {
			deleteUntil = notAfter
		}

		for _, r := range j.flushGrants {
			t.Logf("grant flush check: %s", r.grant.Issuer)
			_, err = gr.GetConcreteGrant(ctx, r.grant.ID)

			if deleteUntil.After(r.grant.ExpiresAt) {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
		}
	}
}

func (j *JanitorConsentTestHelper) LimitSetup(ctx context.Context, reg interface {
	consent.ManagerProvider
	client.ManagerProvider
	flow.CipherProvider
}) func(t *testing.T) {
	cl := reg.ClientManager()
	cm := reg.ConsentManager()

	return func(t *testing.T) {
		var (
			err error
			f   *flow.Flow
		)

		// Create login requests
		for _, r := range j.flushLoginRequests {
			require.NoError(t, cl.CreateClient(ctx, r.Client))
			f, err = cm.CreateLoginRequest(ctx, r)
			require.NoError(t, err)

			// Reject each request
			f.RequestedAt = time.Now() // we won't handle expired flows
			f.LoginAuthenticatedAt = r.AuthenticatedAt
			challenge := x.Must(f.ToLoginChallenge(ctx, reg))

			_, err = cm.HandleLoginRequest(ctx, f, challenge, consent.NewHandledLoginRequest(
				r.ID, true, r.RequestedAt, r.AuthenticatedAt))
			require.NoError(t, err)
		}
	}
}

func (j *JanitorConsentTestHelper) LimitValidate(ctx context.Context, cm consent.Manager) func(t *testing.T) {
	return func(t *testing.T) {
		// flush-login-2 and 3 should be cleared now
		for _, r := range j.flushLoginRequests {
			t.Logf("check login: %s", r.ID)
			_, err := cm.GetLoginRequest(ctx, r.ID)
			// No Requests should have been persisted.
			require.Error(t, err)
		}
	}
}

func (j *JanitorConsentTestHelper) LoginConsentNotAfterSetup(ctx context.Context, cm consent.Manager, cl client.Manager) func(t *testing.T) {
	return func(t *testing.T) {
		var (
			f   *flow.Flow
			err error
		)
		for _, r := range j.flushLoginRequests {
			require.NoError(t, cl.CreateClient(ctx, r.Client))
			f, err = cm.CreateLoginRequest(ctx, r)
			require.NoError(t, err)
		}

		for _, r := range j.flushConsentRequests {
			f.ID = r.LoginChallenge.String()
		}
	}
}

func (j *JanitorConsentTestHelper) LoginConsentNotAfterValidate(
	ctx context.Context,
	notAfter time.Time,
	consentRequestLifespan time.Time,
	reg interface {
		consent.ManagerProvider
		flow.CipherProvider
	},
) func(t *testing.T) {
	return func(t *testing.T) {
		var (
			err error
			f   *flow.Flow
		)

		for _, r := range j.flushLoginRequests {
			isExpired := r.RequestedAt.Before(consentRequestLifespan)
			t.Logf("login flush check:\nNotAfter: %s\nLoginRequest: %s\nis expired: %v\n%+v\n",
				notAfter.String(), consentRequestLifespan.String(), isExpired, r)

			f = x.Must(reg.ConsentManager().CreateLoginRequest(ctx, r))
			loginChallenge := x.Must(f.ToLoginChallenge(ctx, reg))

			_, err = reg.ConsentManager().GetLoginRequest(ctx, loginChallenge)
			// if the lowest between notAfter and consent-request-lifespan is greater than requested_at
			// then the it should expect the value to be deleted.
			if isExpired {
				// value has been deleted here
				require.Error(t, err)
			} else {
				// value has not been deleted here
				require.NoError(t, err)
			}
		}

		for _, r := range j.flushConsentRequests {
			isExpired := r.RequestedAt.Before(consentRequestLifespan)
			t.Logf("consent flush check:\nNotAfter: %s\nConsentRequest: %s\nis expired: %v\n%+v\n",
				notAfter.String(), consentRequestLifespan.String(), isExpired, r)

			f.ID = r.LoginChallenge.String()
			f.RequestedAt = r.RequestedAt
			consentChallenge := x.Must(f.ToConsentChallenge(ctx, reg))

			_, err = reg.ConsentManager().GetConsentRequest(ctx, consentChallenge)
			// if the lowest between notAfter and consent-request-lifespan is greater than requested_at
			// then the it should expect the value to be deleted.
			if isExpired {
				// value has been deleted here
				require.Error(t, err)
			} else {
				// value has not been deleted here
				require.NoError(t, err)
			}
		}
	}
}

func (j *JanitorConsentTestHelper) GetConsentRequestLifespan(ctx context.Context) time.Duration {
	return j.conf.ConsentRequestMaxAge(ctx)
}

func (j *JanitorConsentTestHelper) GetAccessTokenLifespan(ctx context.Context) time.Duration {
	return j.conf.GetAccessTokenLifespan(ctx)
}

func (j *JanitorConsentTestHelper) GetRefreshTokenLifespan(ctx context.Context) time.Duration {
	return j.conf.GetRefreshTokenLifespan(ctx)
}

func (j *JanitorConsentTestHelper) notAfterCheck(notAfter time.Time, lifespan time.Time, requestedAt time.Time) bool {
	// The database deletes where requested_at time is smaller than the lowest between notAfter and consent-request-lifespan
	// thus we get the lowest value here first to compare later to requested_at
	var lesser time.Time
	// if the lowest between notAfter and consent-request-lifespan is greater than requested_at
	// then the it should expect the value to be deleted.
	if notAfter.Unix() < lifespan.Unix() {
		lesser = notAfter
	} else {
		lesser = lifespan
	}

	// true: value has been deleted
	// false: value still exists
	return lesser.Unix() > requestedAt.Unix()
}

func JanitorTests(
	reg interface {
		ConsentManager() consent.Manager
		OAuth2Storage() x.FositeStorer
		config.Provider
		client.ManagerProvider
		flow.CipherProvider
	},
	network string,
	parallel bool,
) func(t *testing.T) {
	return func(t *testing.T) {
		consentManager := reg.ConsentManager()
		clientManager := reg.ClientManager()
		fositeManager := reg.OAuth2Storage()

		if parallel {
			t.Parallel()
		}

		jt := NewConsentJanitorTestHelper(t, network+t.Name())

		ctx := contextx.WithConfigValue(t.Context(), config.KeyConsentRequestMaxAge, jt.GetConsentRequestLifespan(t.Context()))

		t.Run("case=flush-consent-request-not-after", func(t *testing.T) {
			for k, v := range NotAfterTestCycles {
				jt := NewConsentJanitorTestHelper(t, network+k)
				t.Run(fmt.Sprintf("case=%s", k), func(t *testing.T) {
					notAfter := time.Now().Round(time.Second).Add(-v)
					consentRequestLifespan := time.Now().Round(time.Second).Add(-jt.GetConsentRequestLifespan(ctx))

					// setup test
					t.Run("step=setup", jt.LoginConsentNotAfterSetup(ctx, consentManager, clientManager))

					// run the cleanup routine
					t.Run("step=cleanup", func(t *testing.T) {
						require.NoError(t, fositeManager.FlushInactiveLoginConsentRequests(ctx, notAfter, 1000, 100))
					})

					// validate test
					t.Run("step=validate", jt.LoginConsentNotAfterValidate(ctx, notAfter, consentRequestLifespan, reg))
				})

			}
		})

		t.Run("case=flush-consent-request-limit", func(t *testing.T) {
			jt := NewConsentJanitorTestHelper(t, network+"limit")

			t.Run("case=limit", func(t *testing.T) {
				// setup
				t.Run("step=setup", jt.LimitSetup(ctx, reg))

				// cleanup
				t.Run("step=cleanup", func(t *testing.T) {
					require.NoError(t, fositeManager.FlushInactiveLoginConsentRequests(ctx, time.Now().Round(time.Second), 2, 1))
				})

				// validate
				t.Run("step=validate", jt.LimitValidate(ctx, consentManager))
			})
		})
	}
}

func getAccessRequests(uniqueName string, lifespan time.Duration) []*fosite.Request {
	return []*fosite.Request{
		{
			ID:             fmt.Sprintf("%s_flush-access-1", uniqueName),
			RequestedAt:    time.Now().Round(time.Second),
			Client:         &client.Client{ID: fmt.Sprintf("%s_flush-access-1", uniqueName)},
			RequestedScope: fosite.Arguments{"fa", "ba"},
			GrantedScope:   fosite.Arguments{"fa", "ba"},
			Form:           url.Values{"foo": []string{"bar", "baz"}},
			Session:        &oauth2.Session{DefaultSession: &openid.DefaultSession{Subject: "bar"}},
		},
		{
			ID:             fmt.Sprintf("%s_flush-access-2", uniqueName),
			RequestedAt:    time.Now().Round(time.Second).Add(-(lifespan + time.Minute)),
			Client:         &client.Client{ID: fmt.Sprintf("%s_flush-access-2", uniqueName)},
			RequestedScope: fosite.Arguments{"fa", "ba"},
			GrantedScope:   fosite.Arguments{"fa", "ba"},
			Form:           url.Values{"foo": []string{"bar", "baz"}},
			Session:        &oauth2.Session{DefaultSession: &openid.DefaultSession{Subject: "bar"}},
		},
		{
			ID:             fmt.Sprintf("%s_flush-access-3", uniqueName),
			RequestedAt:    time.Now().Round(time.Second).Add(-(lifespan + time.Hour)),
			Client:         &client.Client{ID: fmt.Sprintf("%s_flush-access-3", uniqueName)},
			RequestedScope: fosite.Arguments{"fa", "ba"},
			GrantedScope:   fosite.Arguments{"fa", "ba"},
			Form:           url.Values{"foo": []string{"bar", "baz"}},
			Session:        &oauth2.Session{DefaultSession: &openid.DefaultSession{Subject: "bar"}},
		},
	}
}

func getRefreshRequests(uniqueName string, lifespan time.Duration) []*fosite.AccessRequest {
	var tokenSignature = "4c7c7e8b3a77ad0c3ec846a21653c48b45dbfa31" //nolint:gosec
	return []*fosite.AccessRequest{
		{
			GrantTypes: []string{
				"refresh_token",
			},
			Request: fosite.Request{
				RequestedAt:    time.Now().Round(time.Second),
				ID:             fmt.Sprintf("%s_flush-refresh-1", uniqueName),
				Client:         &client.Client{ID: fmt.Sprintf("%s_flush-refresh-1", uniqueName)},
				RequestedScope: []string{"offline"},
				GrantedScope:   []string{"offline"},
				Session:        &oauth2.Session{DefaultSession: &openid.DefaultSession{Subject: "bar"}},
				Form: url.Values{
					"refresh_token": []string{fmt.Sprintf("%s.%s", fmt.Sprintf("%s_flush-refresh-1", uniqueName), tokenSignature)},
				},
			},
		},
		{
			GrantTypes: []string{
				"refresh_token",
			},
			Request: fosite.Request{
				RequestedAt:    time.Now().Round(time.Second).Add(-(lifespan + time.Minute)),
				ID:             fmt.Sprintf("%s_flush-refresh-2", uniqueName),
				Client:         &client.Client{ID: fmt.Sprintf("%s_flush-refresh-2", uniqueName)},
				RequestedScope: []string{"offline"},
				GrantedScope:   []string{"offline"},
				Session:        &oauth2.Session{DefaultSession: &openid.DefaultSession{Subject: "bar"}},
				Form: url.Values{
					"refresh_token": []string{fmt.Sprintf("%s.%s", fmt.Sprintf("%s_flush-refresh-2", uniqueName), tokenSignature)},
				},
			},
		},
		{
			GrantTypes: []string{
				"refresh_token",
			},
			Request: fosite.Request{
				RequestedAt:    time.Now().Round(time.Second).Add(-(lifespan + time.Hour)),
				ID:             fmt.Sprintf("%s_flush-refresh-3", uniqueName),
				Client:         &client.Client{ID: fmt.Sprintf("%s_flush-refresh-3", uniqueName)},
				RequestedScope: []string{"offline"},
				GrantedScope:   []string{"offline"},
				Session:        &oauth2.Session{DefaultSession: &openid.DefaultSession{Subject: "bar"}},
				Form: url.Values{
					"refresh_token": []string{fmt.Sprintf("%s.%s", fmt.Sprintf("%s_flush-refresh-3", uniqueName), tokenSignature)},
				},
			},
		},
	}
}

func genLoginRequests(uniqueName string, lifespan time.Duration) []*flow.LoginRequest {
	return []*flow.LoginRequest{
		{
			ID:             fmt.Sprintf("%s_flush-login-1", uniqueName),
			RequestedScope: []string{"foo", "bar"},
			Subject:        fmt.Sprintf("%s_flush-login-1", uniqueName),
			Client: &client.Client{
				ID:           fmt.Sprintf("%s_flush-login-consent-1", uniqueName),
				RedirectURIs: []string{"http://redirect"},
			},
			RequestURL:      "http://redirect",
			RequestedAt:     time.Now().Round(time.Second),
			AuthenticatedAt: sqlxx.NullTime(time.Now().Round(time.Second)),
			Verifier:        fmt.Sprintf("%s_flush-login-1", uniqueName),
		},
		{
			ID:             fmt.Sprintf("%s_flush-login-2", uniqueName),
			RequestedScope: []string{"foo", "bar"},
			Subject:        fmt.Sprintf("%s_flush-login-2", uniqueName),
			Client: &client.Client{
				ID:           fmt.Sprintf("%s_flush-login-consent-2", uniqueName),
				RedirectURIs: []string{"http://redirect"},
			},
			RequestURL:      "http://redirect",
			RequestedAt:     time.Now().Round(time.Second).Add(-(lifespan + 10*time.Minute)),
			AuthenticatedAt: sqlxx.NullTime(time.Now().Round(time.Second).Add(-(lifespan + 10*time.Minute))),
			Verifier:        fmt.Sprintf("%s_flush-login-2", uniqueName),
		},
		{
			ID:             fmt.Sprintf("%s_flush-login-3", uniqueName),
			RequestedScope: []string{"foo", "bar"},
			Subject:        fmt.Sprintf("%s_flush-login-3", uniqueName),
			Client: &client.Client{
				ID:           fmt.Sprintf("%s_flush-login-consent-3", uniqueName),
				RedirectURIs: []string{"http://redirect"},
			},
			RequestURL:      "http://redirect",
			RequestedAt:     time.Now().Round(time.Second).Add(-(lifespan + time.Hour)),
			AuthenticatedAt: sqlxx.NullTime(time.Now().Round(time.Second).Add(-(lifespan + time.Hour))),
			Verifier:        fmt.Sprintf("%s_flush-login-3", uniqueName),
		},
	}
}

func genConsentRequests(uniqueName string, lifespan time.Duration) []*flow.OAuth2ConsentRequest {
	return []*flow.OAuth2ConsentRequest{
		{
			ConsentRequestID:     fmt.Sprintf("%s_flush-consent-1", uniqueName),
			RequestedScope:       []string{"foo", "bar"},
			Subject:              fmt.Sprintf("%s_flush-consent-1", uniqueName),
			OpenIDConnectContext: nil,
			ClientID:             fmt.Sprintf("%s_flush-login-consent-1", uniqueName),
			RequestURL:           "http://redirect",
			LoginChallenge:       sqlxx.NullString(fmt.Sprintf("%s_flush-login-1", uniqueName)),
			RequestedAt:          time.Now().Round(time.Second),
			Verifier:             fmt.Sprintf("%s_flush-consent-1", uniqueName),
			CSRF:                 fmt.Sprintf("%s_flush-consent-1", uniqueName),
		},
		{
			ConsentRequestID:     fmt.Sprintf("%s_flush-consent-2", uniqueName),
			RequestedScope:       []string{"foo", "bar"},
			Subject:              fmt.Sprintf("%s_flush-consent-2", uniqueName),
			OpenIDConnectContext: nil,
			ClientID:             fmt.Sprintf("%s_flush-login-consent-2", uniqueName),
			RequestURL:           "http://redirect",
			LoginChallenge:       sqlxx.NullString(fmt.Sprintf("%s_flush-login-2", uniqueName)),
			RequestedAt:          time.Now().Round(time.Second).Add(-(lifespan + time.Minute)),
			Verifier:             fmt.Sprintf("%s_flush-consent-2", uniqueName),
			CSRF:                 fmt.Sprintf("%s_flush-consent-2", uniqueName),
		},
		{
			ConsentRequestID:     fmt.Sprintf("%s_flush-consent-3", uniqueName),
			RequestedScope:       []string{"foo", "bar"},
			Subject:              fmt.Sprintf("%s_flush-consent-3", uniqueName),
			OpenIDConnectContext: nil,
			ClientID:             fmt.Sprintf("%s_flush-login-consent-3", uniqueName),
			RequestURL:           "http://redirect",
			LoginChallenge:       sqlxx.NullString(fmt.Sprintf("%s_flush-login-3", uniqueName)),
			RequestedAt:          time.Now().Round(time.Second).Add(-(lifespan + time.Hour)),
			Verifier:             fmt.Sprintf("%s_flush-consent-3", uniqueName),
			CSRF:                 fmt.Sprintf("%s_flush-consent-3", uniqueName),
		},
	}
}

func getGrantRequests(uniqueName string, lifespan time.Duration) []*createGrantRequest {
	return []*createGrantRequest{
		{
			grant: trust.Grant{
				ID:      uuid.Must(uuid.NewV4()),
				Issuer:  fmt.Sprintf("%s_flush-grant-iss-1", uniqueName),
				Subject: fmt.Sprintf("%s_flush-grant-sub-1", uniqueName),
				Scope:   []string{"foo", "bar"},
				PublicKey: trust.PublicKey{
					Set:   fmt.Sprintf("%s_flush-grant-iss-1", uniqueName),
					KeyID: fmt.Sprintf("%s_flush-grant-kid-1", uniqueName),
				},
				CreatedAt: time.Now().Round(time.Second),
				ExpiresAt: time.Now().Round(time.Second).Add(lifespan),
			},
			pk: jose.JSONWebKey{
				Key:   []byte("asdf"),
				KeyID: fmt.Sprintf("%s_flush-grant-kid-1", uniqueName),
			},
		},
		{
			grant: trust.Grant{
				ID:      uuid.Must(uuid.NewV4()),
				Issuer:  fmt.Sprintf("%s_flush-grant-iss-2", uniqueName),
				Subject: fmt.Sprintf("%s_flush-grant-sub-2", uniqueName),
				Scope:   []string{"foo", "bar"},
				PublicKey: trust.PublicKey{
					Set:   fmt.Sprintf("%s_flush-grant-iss-2", uniqueName),
					KeyID: fmt.Sprintf("%s_flush-grant-kid-2", uniqueName),
				},
				CreatedAt: time.Now().Round(time.Second).Add(-(lifespan + time.Minute)),
				ExpiresAt: time.Now().Round(time.Second).Add(-(lifespan + time.Minute)).Add(lifespan),
			},
			pk: jose.JSONWebKey{
				Key:   []byte("asdf"),
				KeyID: fmt.Sprintf("%s_flush-grant-kid-2", uniqueName),
			},
		},
		{
			grant: trust.Grant{
				ID:      uuid.Must(uuid.NewV4()),
				Issuer:  fmt.Sprintf("%s_flush-grant-iss-3", uniqueName),
				Subject: fmt.Sprintf("%s_flush-grant-sub-3", uniqueName),
				Scope:   []string{"foo", "bar"},
				PublicKey: trust.PublicKey{
					Set:   fmt.Sprintf("%s_flush-grant-iss-3", uniqueName),
					KeyID: fmt.Sprintf("%s_flush-grant-kid-3", uniqueName),
				},
				CreatedAt: time.Now().Round(time.Second).Add(-(lifespan + time.Hour)),
				ExpiresAt: time.Now().Round(time.Second).Add(-(lifespan + time.Hour)).Add(lifespan),
			},
			pk: jose.JSONWebKey{
				Key:   []byte("asdf"),
				KeyID: fmt.Sprintf("%s_flush-grant-kid-3", uniqueName),
			},
		},
	}
}
