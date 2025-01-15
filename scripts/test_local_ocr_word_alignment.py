import io
import json

import pdfplumber
import cv2
import numpy as np
import matplotlib.pyplot as plt


def plot_one_box(x, im, color=(128, 128, 128), label=None, line_thickness=3):
    # Plots one bounding box on image 'im' using OpenCV
    # assert im.data.contiguous, 'Image not contiguous. Apply np.ascontiguousarray(im) to plot_on_box() input image.'
    tl = line_thickness
    c1, c2 = (int(x[0]), int(x[1])), (int(x[2]), int(x[3]))
    # cv2.rectangle(im, c1, c2, color, thickness=tl, lineType=cv2.LINE_AA)
    cv2.rectangle(im, c1, c2, color, lineType=cv2.LINE_AA)
    if label:
        tf = max(tl - 1, 1)  # font thickness
        t_size = cv2.getTextSize(label, 0, fontScale=tl / 3, thickness=tf)[0]
        c2 = c1[0] + t_size[0], c1[1] - t_size[1] - 3
        cv2.rectangle(im, c1, c2, color, -1, cv2.LINE_AA)  # filled
        cv2.putText(im, label, (c1[0], c1[1] - 2), 0, tl / 3, [225, 255, 255], thickness=tf, lineType=cv2.LINE_AA)


if __name__ == "__main__":

    with pdfplumber.open("/Users/jakub/Downloads/3c646447-e936-4b60-bce4-8a5130b7149d.pdf") as pdf:
        with open("./test-ocr.json", "r") as jsonFile:
            ocr = json.load(jsonFile)
            page = pdf.pages[1]

            img = page.to_image().original
            img_byte_arr = io.BytesIO()
            img.save(img_byte_arr, format='JPEG')

            data = np.array(img)

            for word in ocr:
                plot_one_box(
                    [
                        int(word['x0'],),
                        int(word['y0'],),
                        int(word['x1'],),
                        int(word['y1'],),
                    ],
                    data,
                    color=(128, 128, 128),
                    line_thickness=1
                )


            plt.rcParams["figure.dpi"] = 300
            plt.imshow(data)
            plt.show()
