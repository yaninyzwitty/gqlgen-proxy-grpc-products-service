CREATE TABLE IF NOT EXISTS categories (
    id bigint primary key,
    name text,
    description text,
    created_at timestamp
);


CREATE TABLE IF NOT EXISTS products (
  id bigint ,
  category_id bigint,
  name text,
  description text,
  price float,
  stock int,
  created_at timestamp,
  updated_at timestamp,
  PRIMARY KEY ((category_id), id)
) WITH CLUSTERING ORDER BY (id DESC);


CREATE TABLE IF NOT EXISTS products_outbox (
    id uuid,
    bucket text,
    payload text,
    event_type text,
    PRIMARY KEY((bucket), id)
);

CREATE TABLE inventory (
    product_id bigint,
    category_id bigint,
    stock_count int,
    created_at timestamp,
    last_updated_at timestamp,
    PRIMARY KEY (product_id, category_id)
);
