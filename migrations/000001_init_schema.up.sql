CREATE TABLE hospitals (
    id           UUID PRIMARY KEY,
    code         VARCHAR(50)  NOT NULL UNIQUE,
    name         VARCHAR(255) NOT NULL,
    his_base_url VARCHAR(255),
    created_at   TIMESTAMPTZ  NOT NULL DEFAULT now()
);

CREATE TABLE staff (
    id            UUID PRIMARY KEY,
    hospital_id   UUID         NOT NULL REFERENCES hospitals (id),
    username      VARCHAR(100) NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    created_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at    TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT staff_hospital_username_key UNIQUE (hospital_id, username)
);

CREATE TABLE patients (
    id             UUID PRIMARY KEY,
    hospital_id    UUID         NOT NULL REFERENCES hospitals (id),
    patient_hn     VARCHAR(50)  NOT NULL,
    national_id    VARCHAR(20),
    passport_id    VARCHAR(20),
    first_name_th  VARCHAR(100),
    middle_name_th VARCHAR(100),
    last_name_th   VARCHAR(100),
    first_name_en  VARCHAR(100),
    middle_name_en VARCHAR(100),
    last_name_en   VARCHAR(100),
    date_of_birth  DATE,
    phone_number   VARCHAR(30),
    email          VARCHAR(255),
    gender         CHAR(1),
    created_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    updated_at     TIMESTAMPTZ  NOT NULL DEFAULT now(),
    CONSTRAINT patients_hospital_hn_key UNIQUE (hospital_id, patient_hn),
    CONSTRAINT patients_gender_check CHECK (gender IN ('M', 'F')),
    CONSTRAINT patients_identifier_check CHECK (national_id IS NOT NULL OR passport_id IS NOT NULL)
);

-- One patient per identifier within a hospital; NULLs are not indexed.
CREATE UNIQUE INDEX patients_hospital_national_id_key
    ON patients (hospital_id, national_id) WHERE national_id IS NOT NULL;
CREATE UNIQUE INDEX patients_hospital_passport_id_key
    ON patients (hospital_id, passport_id) WHERE passport_id IS NOT NULL;

CREATE INDEX patients_hospital_dob_idx ON patients (hospital_id, date_of_birth);
