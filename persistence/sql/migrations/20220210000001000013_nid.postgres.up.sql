-- Migration generated by the command below; DO NOT EDIT.
-- ./hydra migrate gen ./persistence/sql/src/20220210000001_nid/ ./persistence/sql/migrations/

ALTER TABLE hydra_oauth2_authentication_session ALTER nid SET NOT NULL;
