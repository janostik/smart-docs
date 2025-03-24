create table if not exists documents
(
    id           integer primary key,
    name         text,
    status       text,
    upload_date  datetime,
    ocr_required bool,
    page_count   integer,
    mode         text not null default 'manual' check (mode in ('mistral', 'manual')),
    mistral_file_id text
);

create table if not exists pages
(
    id          integer primary key,
    document_id integer,
    page_num    integer,
    pdf_text    text,
    ocr_text    text,
    status      text,
    predictions text,
    html        text,
    md          text,
    width       integer,
    height      integer,
    foreign key (document_id) references documents (id) on delete cascade
);
