# extract_text_with_bboxes.py
import pdfplumber
import sys
import json


def extract_text_with_bboxes(path):
    with pdfplumber.open(path) as pdf:
        doc_words = []
        for page_num, page in enumerate(pdf.pages):
            page_words = []
            words = page.extract_words()
            for w in words:
                page_words.append({
                    "bbox": [
                        w['x0'],
                        w['top'],
                        w['x1'],
                        w['bottom']
                    ],
                    "text": w['text']
                })
            doc_words.append(page_words)

    return doc_words


if __name__ == "__main__":
    pdf_path = sys.argv[1]
    data = extract_text_with_bboxes(pdf_path)
    print(json.dumps(data))
