INSERT INTO hydra_client (id,
                          nid,
                          client_name,
                          client_secret,
                          redirect_uris,
                          grant_types,
                          response_types,
                          scope,
                          owner,
                          policy_uri,
                          tos_uri,
                          client_uri,
                          logo_uri,
                          contacts,
                          client_secret_expires_at,
                          sector_identifier_uri,
                          jwks,
                          jwks_uri,
                          request_uris,
                          token_endpoint_auth_method,
                          request_object_signing_alg,
                          userinfo_signed_response_alg,
                          subject_type,
                          allowed_cors_origins,
                          pk_deprecated,
                          audience,
                          created_at,
                          updated_at,
                          frontchannel_logout_uri,
                          frontchannel_logout_session_required,
                          post_logout_redirect_uris,
                          backchannel_logout_uri,
                          backchannel_logout_session_required,
                          metadata,
                          token_endpoint_auth_signing_alg,
                          pk,
                          registration_access_token_signature,
                          skip_consent,
                          skip_logout_consent,
                          device_authorization_grant_id_token_lifespan,
                          device_authorization_grant_access_token_lifespan,
                          device_authorization_grant_refresh_token_lifespan)
VALUES ('client-23',
        (SELECT id FROM networks LIMIT 1), 'Client 23', 'secret-23', '["http://redirect/23_1","http://redirect/23_2"]', '["grant-23_1","grant-23_2"]', '["response-23_1","response-23_2"]', 'scope-23', 'owner-23', 'http://policy/23', 'http://tos/23', 'http://client/23', 'http://logo/23', '["contact-23_1","contact-23_2"]', 0, 'http://sector_id/23', '', 'http://jwks/23', '["http://request/23_1","http://request/23_2"]', 'token_auth-23', 'r_alg-23', 'u_alg-23', 'subject-23', '["http://cors/23_1","http://cors/23_2"]', 0, '["autdience-23_1","autdience-23_2"]', '2023-02-15 23:20:23.004598', '2023-02-15 23:20:23.004598', 'http://front_logout/23', true, '["http://post_redirect/23_1","http://post_redirect/23_2"]', 'http://back_logout/23', true, '{"migration": "23"}', '', '52f38352-7944-4ace-b55c-5aded28f4ba6', '', TRUE, TRUE, 3600, 3600, 3600);


INSERT INTO hydra_oauth2_flow (login_challenge,
                               nid,
                               requested_scope,
                               login_verifier,
                               login_csrf,
                               subject,
                               request_url,
                               login_skip,
                               client_id,
                               requested_at,
                               oidc_context,
                               login_session_id,
                               requested_at_audience,
                               login_initialized_at,
                               state,
                               login_remember,
                               login_remember_for,
                               login_error,
                               acr,
                               login_authenticated_at,
                               login_was_used,
                               forced_subject_identifier,
                               context,
                               amr,
                               consent_challenge_id,
                               consent_verifier,
                               consent_skip,
                               consent_csrf,
                               granted_scope,
                               consent_remember,
                               consent_remember_for,
                               consent_error,
                               session_access_token,
                               session_id_token,
                               consent_was_used,
                               granted_at_audience,
                               consent_handled_at,
                               login_extend_session_lifespan,
                               device_challenge_id,
                               device_code_request_id,
                               device_verifier,
                               device_csrf,
                               device_was_used,
                               device_handled_at,
                               device_error)
VALUES ('challenge-0018',
        (SELECT id FROM networks LIMIT 1), '["requested_scope-0018_1","requested_scope-0018_2"]', 'verifier-0018', 'csrf-0018', 'subject-0018', 'http://request/0018', true, 'client-21', CURRENT_TIMESTAMP, '{"display": "display-0018"}', NULL, '["requested_audience-0018_1","requested_audience-0018_2"]', CURRENT_TIMESTAMP, 128, true, 15, '{}', 'acr-0018', CURRENT_TIMESTAMP, true, 'force_subject_id-0018', '{"context": "0018"}', '["amr-0018-1","amr-0018-2"]', 'challenge-0018', 'verifier-0018', true, 'csrf-0018', '["granted_scope-0018_1","granted_scope-0018_2"]', true, 15, '{}', '{"session_access_token-0018": "0018"}', '{"session_id_token-0018": "0018"}', true, '["granted_audience-0018_1","granted_audience-0018_2"]', '2025-05-16 12:24', true, 'device-challenge-0018', 'device-request-id-0018', 'device-verifier-0018', 'device-csrf-0018', true, '2025-05-16 12:24', '{}' );

INSERT INTO hydra_oauth2_device_auth_codes (device_code_signature, user_code_signature, request_id, requested_at,
                                            client_id, scope, granted_scope, form_data, session_data, subject,
                                            device_code_active, user_code_state, requested_audience, granted_audience,
                                            challenge_id, expires_at, nid)
VALUES ('device-code-signature-0001', 'user-code-signature-0001', 'request-id-0001', '2025-05-16 12:24',
        'client-21', '["scope-0001_1","scope-0001_2"]', '["granted_scope-0001_1","granted_scope-0001_2"]',
        '{"form_data": "0001"}',
        '{"session_data": "0001"}', 'subject-0001', true, 0,
        '["requested_audience-0001_1","requested_audience-0001_2"]',
        '["granted_audience-0001_1","granted_audience-0001_2"]', 'challenge-0018', '2025-05-16 12:24',
        (SELECT id FROM networks LIMIT 1)
  );
