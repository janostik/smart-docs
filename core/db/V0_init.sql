create table if not exists documents
(
    id           integer primary key,
    name         text,
    status       text,
    upload_date  datetime,
    ocr_required bool,
    page_count   integer
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
    width       integer,
    height      integer,
    foreign key (document_id) references documents (id) on delete cascade
);
