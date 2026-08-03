CREATE TABLE public.urls (
	urls_short_url varchar(255) NOT NULL,
	urls_original_url varchar NOT NULL,
	CONSTRAINT urls_pk PRIMARY KEY (urls_short_url)
);