CREATE TABLE users(
    id serial primary key,
    name varchar(32) not null,
    password varchar(255) not null,
    password_hash varchar(255) not null,
    created_at timestamp default current_timestamp
);

CREATE TABLE pastas(
    id serial primary key,
    hash text not null unique,
    user_id int,
    storage_key text not null unique,
    size int,
    created_at timestamp default current_timestamp,
    expired_at timestamp
);