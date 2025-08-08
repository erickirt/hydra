// Copyright © 2022 Ory Corp
// SPDX-License-Identifier: Apache-2.0

package testhelpers

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/tidwall/gjson"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"golang.org/x/oauth2"

	"github.com/ory/fosite/token/jwt"
	"github.com/ory/hydra/v2/client"
	"github.com/ory/hydra/v2/driver"
	"github.com/ory/hydra/v2/driver/config"
	"github.com/ory/hydra/v2/x"
	"github.com/ory/x/contextx"
	"github.com/ory/x/httpx"
	"github.com/ory/x/ioutilx"
)

func NewIDToken(t *testing.T, reg driver.Registry, subject string) string {
	return NewIDTokenWithExpiry(t, reg, subject, time.Hour)
}

func NewIDTokenWithExpiry(t *testing.T, reg driver.Registry, subject string, exp time.Duration) string {
	token, _, err := reg.OpenIDJWTStrategy().Generate(context.Background(), jwt.IDTokenClaims{
		Subject:   subject,
		ExpiresAt: time.Now().Add(exp),
		IssuedAt:  time.Now(),
	}.ToMapClaims(), jwt.NewHeaders())
	require.NoError(t, err)
	return token
}

func NewIDTokenWithClaims(t *testing.T, reg driver.Registry, claims jwt.MapClaims) string {
	token, _, err := reg.OpenIDJWTStrategy().Generate(context.Background(), claims, jwt.NewHeaders())
	require.NoError(t, err)
	return token
}

// NewOAuth2Server
// Deprecated: use NewConfigurableOAuth2Server instead
func NewOAuth2Server(ctx context.Context, t testing.TB, reg driver.Registry) (publicTS, adminTS *httptest.Server) {
	reg.Config().MustSet(ctx, config.KeySubjectIdentifierAlgorithmSalt, "76d5d2bf-747f-4592-9fbd-d2b895a54b3a")
	reg.Config().MustSet(ctx, config.KeyAccessTokenLifespan, 10*time.Second)
	reg.Config().MustSet(ctx, config.KeyRefreshTokenLifespan, 20*time.Second)
	reg.Config().MustSet(ctx, config.KeyScopeStrategy, "exact")

	public, admin := NewConfigurableOAuth2Server(ctx, t, reg)
	return public.Server, admin.Server
}

func NewConfigurableOAuth2Server(ctx context.Context, t testing.TB, reg driver.Registry) (publicTS, adminTS *contextx.ConfigurableTestServer) {
	public, admin := x.NewRouterPublic(), x.NewRouterAdmin(reg.Config().AdminURL)

	MustEnsureRegistryKeys(ctx, reg, x.OpenIDConnectKeyName)
	MustEnsureRegistryKeys(ctx, reg, x.OAuth2JWTKeyName)

	reg.RegisterPublicRoutes(ctx, public)
	reg.RegisterAdminRoutes(admin)

	publicTS = contextx.NewConfigurableTestServer(otelhttp.NewHandler(public, "public", otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
		return r.URL.Path
	})))
	t.Cleanup(publicTS.Close)

	adminTS = contextx.NewConfigurableTestServer(otelhttp.NewHandler(admin, "admin", otelhttp.WithSpanNameFormatter(func(_ string, r *http.Request) string {
		return r.URL.Path
	})))
	t.Cleanup(adminTS.Close)

	reg.Config().MustSet(ctx, config.KeyIssuerURL, publicTS.URL)
	return publicTS, adminTS
}

func DecodeIDToken(t *testing.T, token *oauth2.Token) gjson.Result {
	idt, ok := token.Extra("id_token").(string)
	require.True(t, ok)
	assert.NotEmpty(t, idt)

	return gjson.ParseBytes(InsecureDecodeJWT(t, idt))
}

func IntrospectToken(t testing.TB, token string, adminTS *httptest.Server) gjson.Result {
	require.NotEmpty(t, token)

	req := httpx.MustNewRequest("POST", adminTS.URL+"/admin/oauth2/introspect",
		strings.NewReader((url.Values{"token": {token}}).Encode()),
		"application/x-www-form-urlencoded")

	res, err := adminTS.Client().Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	require.NoError(t, err)
	require.Equalf(t, http.StatusOK, res.StatusCode, "Response body: %s", body)
	return gjson.ParseBytes(body)
}

func RevokeToken(t testing.TB, conf *oauth2.Config, token string, publicTS *httptest.Server) gjson.Result {
	require.NotEmpty(t, token)

	req := httpx.MustNewRequest("POST", publicTS.URL+"/oauth2/revoke",
		strings.NewReader((url.Values{"token": {token}}).Encode()),
		"application/x-www-form-urlencoded")

	req.SetBasicAuth(conf.ClientID, conf.ClientSecret)
	res, err := publicTS.Client().Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	return gjson.ParseBytes(ioutilx.MustReadAll(res.Body))
}

func UpdateClientTokenLifespans(t *testing.T, conf *oauth2.Config, clientID string, lifespans client.Lifespans, adminTS *httptest.Server) {
	b, err := json.Marshal(lifespans)
	require.NoError(t, err)
	req := httpx.MustNewRequest(
		"PUT",
		adminTS.URL+"/admin"+client.ClientsHandlerPath+"/"+clientID+"/lifespans",
		bytes.NewBuffer(b),
		"application/json",
	)
	req.SetBasicAuth(conf.ClientID, conf.ClientSecret)
	res, err := adminTS.Client().Do(req)
	require.NoError(t, err)
	defer res.Body.Close()
	require.Equal(t, res.StatusCode, http.StatusOK)
}

func Userinfo(t *testing.T, token *oauth2.Token, publicTS *httptest.Server) gjson.Result {
	require.NotEmpty(t, token.AccessToken)

	req := httpx.MustNewRequest("GET", publicTS.URL+"/userinfo", nil, "")
	req.Header.Set("Authorization", "Bearer "+token.AccessToken)

	res, err := publicTS.Client().Do(req)
	require.NoError(t, err)

	defer res.Body.Close()
	return gjson.ParseBytes(ioutilx.MustReadAll(res.Body))
}

func HTTPServerNotImplementedHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusNotImplemented)
}

func HTTPServerNoExpectedCallHandler(t testing.TB) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("This should not have been called")
	}
}

func NewLoginConsentUI(t testing.TB, c *config.DefaultProvider, login, consent http.HandlerFunc) {
	if login == nil {
		login = HTTPServerNotImplementedHandler
	}

	if consent == nil {
		login = HTTPServerNotImplementedHandler
	}

	lt := httptest.NewServer(login)
	ct := httptest.NewServer(consent)

	t.Cleanup(lt.Close)
	t.Cleanup(ct.Close)

	c.MustSet(context.Background(), config.KeyLoginURL, lt.URL)
	c.MustSet(context.Background(), config.KeyConsentURL, ct.URL)
}

func NewDeviceLoginConsentUI(t testing.TB, c *config.DefaultProvider, device, login, consent http.HandlerFunc) {
	if device == nil {
		device = HTTPServerNotImplementedHandler
	}
	dt := httptest.NewServer(device)
	t.Cleanup(dt.Close)
	c.MustSet(context.Background(), config.KeyDeviceVerificationURL, dt.URL)

	NewLoginConsentUI(t, c, login, consent)
}

func NewCallbackURL(t testing.TB, prefix string, h http.HandlerFunc) string {
	if h == nil {
		h = HTTPServerNotImplementedHandler
	}

	r := httprouter.New()
	r.GET("/"+prefix, func(w http.ResponseWriter, r *http.Request, _ httprouter.Params) {
		h(w, r)
	})
	ts := httptest.NewServer(r)
	t.Cleanup(ts.Close)

	return ts.URL + "/" + prefix
}

func NewEmptyCookieJar(t testing.TB) *cookiejar.Jar {
	c, err := cookiejar.New(&cookiejar.Options{})
	require.NoError(t, err)
	return c
}

func NewEmptyJarClient(t testing.TB) *http.Client {
	return &http.Client{
		Jar:       NewEmptyCookieJar(t),
		Transport: &loggingTransport{t},
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			//t.Logf("Redirect to %s", req.URL.String())

			if len(via) >= 20 {
				for k, v := range via {
					t.Logf("Failed with redirect (%d): %s", k, v.URL.String())
				}
				return errors.New("stopped after 20 redirects")
			}
			return nil
		},
	}
}

type loggingTransport struct{ t testing.TB }

func (s *loggingTransport) RoundTrip(r *http.Request) (*http.Response, error) {
	//s.t.Logf("%s %s", r.Method, r.URL.String())
	//s.t.Logf("%s %s\nWith Cookies: %v", r.Method, r.URL.String(), r.Cookies())

	return otelhttp.DefaultClient.Transport.RoundTrip(r)
}

// InsecureDecodeJWT decodes a JWT payload without checking the signature.
func InsecureDecodeJWT(t require.TestingT, token string) []byte {
	parts := strings.Split(token, ".")
	require.Len(t, parts, 3)
	dec, err := base64.RawURLEncoding.DecodeString(parts[1])
	require.NoErrorf(t, err, "failed to decode JWT payload: %s", parts[1])
	return dec
}
