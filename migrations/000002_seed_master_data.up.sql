-- Master data so the API is usable right after the first start.
--
-- /staff/create requires a login, so each hospital gets one initial staff
-- member, "admin". Their password is a DEV-ONLY value (see README); change
-- or delete these rows before any real deployment.
INSERT INTO hospitals (id, code, name, his_base_url) VALUES
    ('a0000000-0000-4000-8000-000000000001', 'hospital-a', 'Hospital A', 'https://hospital-a.api.co.th'),
    ('b0000000-0000-4000-8000-000000000002', 'hospital-b', 'Hospital B', NULL);

INSERT INTO staff (id, hospital_id, username, password_hash) VALUES
    ('a1000000-0000-4000-8000-000000000001', 'a0000000-0000-4000-8000-000000000001', 'admin',
     '$2a$10$ADjbwmzaGpGq9Hg1Eq06cODTNl8xADCM6odpjpQFAQkdf3j8haWfG'),
    ('b1000000-0000-4000-8000-000000000002', 'b0000000-0000-4000-8000-000000000002', 'admin',
     '$2a$10$ADjbwmzaGpGq9Hg1Eq06cODTNl8xADCM6odpjpQFAQkdf3j8haWfG');
