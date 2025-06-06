-- Migration generated by the command below; DO NOT EDIT.
-- ./hydra migrate gen ./persistence/sql/src/20220210000001_nid/ ./persistence/sql/migrations/

CREATE INDEX hydra_oauth2_code_request_id_idx ON hydra_oauth2_code (request_id, nid);



-- hydra_oauth2_flow
ALTER TABLE `hydra_oauth2_flow` ADD COLUMN `nid` char(36);
ALTER TABLE `hydra_oauth2_flow` ADD CONSTRAINT `hydra_oauth2_flow_nid_fk_idx` FOREIGN KEY (`nid`) REFERENCES `networks` (`id`) ON UPDATE RESTRICT ON DELETE CASCADE;
