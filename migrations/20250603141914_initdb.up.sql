CREATE TABLE users(
    id serial primary key,
    name varchar(32) not null,
    email varchar(128) not null unique,
    password_hash varchar(255) not null,
    created_at timestamp default current_timestamp
);

CREATE TABLE pastas(
    id serial primary key,
    user_id int,
    hash text not null unique,
    object_id text not null unique,

    size int,
    language varchar(50) default 'plaintext',
    visibility varchar(50) default 'public',
    views int default 0,
    password_hash text,

    created_at timestamp default current_timestamp,
    expires_at timestamp
);