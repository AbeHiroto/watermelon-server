--
-- PostgreSQL database dump
--

-- Dumped from database version 14.10 (Ubuntu 14.10-0ubuntu0.22.04.1)
-- Dumped by pg_dump version 14.10 (Ubuntu 14.10-0ubuntu0.22.04.1)

SET statement_timeout = 0;
SET lock_timeout = 0;
SET idle_in_transaction_session_timeout = 0;
SET client_encoding = 'UTF8';
SET standard_conforming_strings = on;
SELECT pg_catalog.set_config('search_path', '', false);
SET check_function_bodies = false;
SET xmloption = content;
SET client_min_messages = warning;
SET row_security = off;

SET default_tablespace = '';

SET default_table_access_method = heap;

--
-- Name: game_rooms; Type: TABLE; Schema: public; Owner: satoshixic
--

CREATE TABLE public.game_rooms (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    game_room_id bigint,
    platform text NOT NULL,
    account_name text NOT NULL,
    match_type text NOT NULL,
    unfairness_degree bigint NOT NULL,
    game_state text NOT NULL,
    creation_time bigint NOT NULL,
    last_activity_time bigint,
    finish_time bigint,
    start_time bigint,
    room_theme text
);


ALTER TABLE public.game_rooms OWNER TO satoshixic;

--
-- Name: game_rooms_id_seq; Type: SEQUENCE; Schema: public; Owner: satoshixic
--

CREATE SEQUENCE public.game_rooms_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.game_rooms_id_seq OWNER TO satoshixic;

--
-- Name: game_rooms_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: satoshixic
--

ALTER SEQUENCE public.game_rooms_id_seq OWNED BY public.game_rooms.id;


--
-- Name: users; Type: TABLE; Schema: public; Owner: satoshixic
--

CREATE TABLE public.users (
    id bigint NOT NULL,
    created_at timestamp with time zone,
    updated_at timestamp with time zone,
    deleted_at timestamp with time zone,
    user_id text NOT NULL,
    subscription_status text NOT NULL,
    valid_room_count bigint NOT NULL
);


ALTER TABLE public.users OWNER TO satoshixic;

--
-- Name: users_id_seq; Type: SEQUENCE; Schema: public; Owner: satoshixic
--

CREATE SEQUENCE public.users_id_seq
    START WITH 1
    INCREMENT BY 1
    NO MINVALUE
    NO MAXVALUE
    CACHE 1;


ALTER TABLE public.users_id_seq OWNER TO satoshixic;

--
-- Name: users_id_seq; Type: SEQUENCE OWNED BY; Schema: public; Owner: satoshixic
--

ALTER SEQUENCE public.users_id_seq OWNED BY public.users.id;


--
-- Name: game_rooms id; Type: DEFAULT; Schema: public; Owner: satoshixic
--

ALTER TABLE ONLY public.game_rooms ALTER COLUMN id SET DEFAULT nextval('public.game_rooms_id_seq'::regclass);


--
-- Name: users id; Type: DEFAULT; Schema: public; Owner: satoshixic
--

ALTER TABLE ONLY public.users ALTER COLUMN id SET DEFAULT nextval('public.users_id_seq'::regclass);


--
-- Data for Name: game_rooms; Type: TABLE DATA; Schema: public; Owner: satoshixic
--

COPY public.game_rooms (id, created_at, updated_at, deleted_at, game_room_id, platform, account_name, match_type, unfairness_degree, game_state, creation_time, last_activity_time, finish_time, start_time, room_theme) FROM stdin;
\.


--
-- Data for Name: users; Type: TABLE DATA; Schema: public; Owner: satoshixic
--

COPY public.users (id, created_at, updated_at, deleted_at, user_id, subscription_status, valid_room_count) FROM stdin;
\.


--
-- Name: game_rooms_id_seq; Type: SEQUENCE SET; Schema: public; Owner: satoshixic
--

SELECT pg_catalog.setval('public.game_rooms_id_seq', 1, false);


--
-- Name: users_id_seq; Type: SEQUENCE SET; Schema: public; Owner: satoshixic
--

SELECT pg_catalog.setval('public.users_id_seq', 1, false);


--
-- Name: game_rooms game_rooms_pkey; Type: CONSTRAINT; Schema: public; Owner: satoshixic
--

ALTER TABLE ONLY public.game_rooms
    ADD CONSTRAINT game_rooms_pkey PRIMARY KEY (id);


--
-- Name: users users_pkey; Type: CONSTRAINT; Schema: public; Owner: satoshixic
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_pkey PRIMARY KEY (id);


--
-- Name: users users_user_id_key; Type: CONSTRAINT; Schema: public; Owner: satoshixic
--

ALTER TABLE ONLY public.users
    ADD CONSTRAINT users_user_id_key UNIQUE (user_id);


--
-- Name: idx_game_rooms_deleted_at; Type: INDEX; Schema: public; Owner: satoshixic
--

CREATE INDEX idx_game_rooms_deleted_at ON public.game_rooms USING btree (deleted_at);


--
-- Name: idx_users_deleted_at; Type: INDEX; Schema: public; Owner: satoshixic
--

CREATE INDEX idx_users_deleted_at ON public.users USING btree (deleted_at);


--
-- PostgreSQL database dump complete
--

