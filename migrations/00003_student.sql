-- +goose Up
-- Modul student: siswa, rombel, enrollment, wali, guru, mapel. Lihat docs/03-student.md.

CREATE TABLE students (
    id         bigserial PRIMARY KEY,
    school_id  bigint NOT NULL REFERENCES schools (id),
    nis        text NOT NULL,
    nisn       text,
    name       text NOT NULL,
    gender     text,
    birth_date date,
    user_id    bigint REFERENCES users (id),
    status     text NOT NULL DEFAULT 'active',  -- active|graduated|moved|dropped
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (school_id, nis)
);
CREATE INDEX students_school ON students (school_id);
CREATE INDEX students_school_nisn ON students (school_id, nisn);
CREATE INDEX students_user ON students (user_id);

CREATE TABLE classes (
    id                  bigserial PRIMARY KEY,
    school_id           bigint NOT NULL REFERENCES schools (id),
    academic_year_id    bigint NOT NULL REFERENCES academic_years (id),
    name                text NOT NULL,
    grade               text NOT NULL,
    major               text,
    homeroom_teacher_id bigint,
    UNIQUE (school_id, academic_year_id, name)
);
CREATE INDEX classes_school_year ON classes (school_id, academic_year_id);

CREATE TABLE enrollments (
    id         bigserial PRIMARY KEY,
    school_id  bigint NOT NULL REFERENCES schools (id),
    student_id bigint NOT NULL REFERENCES students (id),
    class_id   bigint NOT NULL REFERENCES classes (id),
    created_at timestamptz NOT NULL DEFAULT now(),
    UNIQUE (student_id, class_id)
);
CREATE INDEX enrollments_school ON enrollments (school_id);
CREATE INDEX enrollments_class ON enrollments (class_id);
CREATE INDEX enrollments_student ON enrollments (student_id);

CREATE TABLE guardians (
    id         bigserial PRIMARY KEY,
    school_id  bigint NOT NULL REFERENCES schools (id),
    user_id    bigint NOT NULL REFERENCES users (id),
    student_id bigint NOT NULL REFERENCES students (id),
    relation   text NOT NULL,  -- ayah|ibu|wali
    UNIQUE (user_id, student_id)
);
CREATE INDEX guardians_school ON guardians (school_id);
CREATE INDEX guardians_student ON guardians (student_id);

CREATE TABLE teachers (
    id        bigserial PRIMARY KEY,
    school_id bigint NOT NULL REFERENCES schools (id),
    user_id   bigint NOT NULL REFERENCES users (id),
    nip       text,
    UNIQUE (school_id, user_id)
);
CREATE INDEX teachers_school ON teachers (school_id);

-- wali kelas: dirujuk setelah teachers ada.
ALTER TABLE classes
    ADD CONSTRAINT classes_homeroom_teacher_fkey
    FOREIGN KEY (homeroom_teacher_id) REFERENCES teachers (id);

CREATE TABLE subjects (
    id        bigserial PRIMARY KEY,
    school_id bigint NOT NULL REFERENCES schools (id),
    code      text NOT NULL,
    name      text NOT NULL,
    UNIQUE (school_id, code)
);
CREATE INDEX subjects_school ON subjects (school_id);

-- +goose Down
DROP TABLE subjects;
ALTER TABLE classes DROP CONSTRAINT classes_homeroom_teacher_fkey;
DROP TABLE teachers;
DROP TABLE guardians;
DROP TABLE enrollments;
DROP TABLE classes;
DROP TABLE students;
