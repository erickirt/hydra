-- Migration generated by the command below; DO NOT EDIT.
-- ./hydra migrate gen ./persistence/sql/src/20220210000001_nid/ ./persistence/sql/migrations/

CREATE INDEX hydra_oauth2_refresh_client_id_idx ON hydra_oauth2_refresh (client_id ASC, nid ASC);
CREATE INDEX hydra_oauth2_refresh_challenge_id_idx ON hydra_oauth2_refresh (challenge_id ASC);
CREATE INDEX hydra_oauth2_refresh_client_id_subject_idx ON hydra_oauth2_refresh (client_id ASC, subject ASC);
CREATE INDEX hydra_oauth2_refresh_request_id_idx ON hydra_oauth2_refresh (request_id ASC);
