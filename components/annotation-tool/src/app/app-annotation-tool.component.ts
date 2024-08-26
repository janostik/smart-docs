import {ChangeDetectorRef, Component, ElementRef, inject, OnInit, ViewChild} from '@angular/core';
import {HttpClient} from "@angular/common/http";
import {Subject, takeUntil, tap} from "rxjs";
import {AppAnnotationComponent} from "./app-annotation.component";

import svgPanZoom from "svg-pan-zoom";
import Instance = SvgPanZoom.Instance;

export interface Annotation {
    score: number,
    label: string,
    x0: number,
    x1: number,
    y0: number,
    y1: number,
    table: Annotation[]
}

@Component({
    selector: 'app-annotation-tool',
    standalone: true,
    imports: [
        AppAnnotationComponent,
    ],
    template: `
        @if (imageUrl) {
            <svg #root>
                <g #viewport>
                    <svg:image x="0" y="0"
                               [attr.href]="imageUrl"
                               width="595px"
                               height="843px"
                               preserveAspectRatio="xMidYMax meet"/>

                    @for (segment of annotations; track segment; let index = $index) {
                        <svg:g app-annotation [id]="index"
                               [attr.id]="'el-' + index"
                               [box]="segment"
                               [fill]="fill(segment)"
                               [rootEl]="rootEl.nativeElement"
                               [viewPortEl]="viewportEl.nativeElement"
                               (rightClicked)="delete(segment)"
                               (selected)="selectSegmentAndElement(segment, $event)"
                               (segmentPositionChanged)="onPositionChanged()"
                        />
                    }
                </g>
            </svg>
            
            <div class="floating">
                @if (isDrawingElement) {
                    <button (click)="isDrawingElement = false">(C)ancel</button>
                } @else {
                    <button (click)="isDrawingElement = true">(C)reate</button>
                }
            </div>
        }
    `,
    styles: `
        :host {
            position: relative;
        }

        svg {
            position: absolute;
            left: 0;
            top: 0;
            width: 100%;
            height: 100%;
        }
        
        .floating {
            position: absolute;
            right: 24px;
            bottom: 24px;
        }
    `
})
export class AppAnnotationToolComponent implements OnInit {
    type: ('DOCUMENT' | 'TABLE') = 'DOCUMENT';
    documentId: string = '';
    pageNumber: string = '';
    imageUrl?: string
    annotations: Annotation[] = [];


    @ViewChild("root") rootEl!: ElementRef<SVGSVGElement>;
    @ViewChild("viewport") viewportEl!: ElementRef<SVGGElement>;


    http = inject(HttpClient)
    loading = false;
    zoomLevel: number = 1;
    activeLabel = "paragraph"

    private _zoomable?: Instance;
    private _rect?: SVGRectElement;
    private _isDrawingElement: boolean = false;
    private _drawStartPoint: { x: number; y: number } = {x: 0, y: 0};
    protected _onDestroy$ = new Subject<void>();

    set isDrawingElement(value: boolean) {
        this._isDrawingElement = value;
        if (this._rect) {
            this._rect.remove()
            this._rect = undefined
        }
        if (this.isDrawingElement && !!this.rootEl) {
            this._zoomable?.disablePan()
            this.rootEl.nativeElement.addEventListener('mousedown', this.drawStart, { passive: true });
        } else if (this.viewportEl) {
            this._zoomable?.enablePan()
            this.rootEl.nativeElement.removeEventListener('mousedown', this.drawStart);
            this.rootEl.nativeElement.removeEventListener('mousemove', this.drawMove);
            this.rootEl.nativeElement.removeEventListener('mouseup', this.drawEnd);
        }
        this._cd.markForCheck();
    }

    get isDrawingElement() {
        return this._isDrawingElement;
    }

    constructor(private elementRef: ElementRef, private _cd: ChangeDetectorRef) {
        this.type = this.elementRef.nativeElement.getAttribute('type');
        this.documentId = this.elementRef.nativeElement.getAttribute('document-id');
        this.pageNumber = this.elementRef.nativeElement.getAttribute('page-number');
        this.imageUrl = `/assets/images/${this.documentId}/${this.pageNumber}.jpg`
    }

    ngOnInit() {
        this.http
            .get<Annotation[]>(`/document/${this.documentId}/${this.pageNumber}/predictions`)
            .pipe(
                takeUntil(this._onDestroy$)
            )
            .subscribe(annotations => {
                this.annotations = annotations
                this._setZoomable();
                this._cd.markForCheck()
            })
    }

    fill(segment: Annotation) {
        switch (segment.label) {
            default:
                return "#ff6600"
        }
    }

    selectSegmentAndElement(prediction: Annotation, $event: MouseEvent) {
        console.log(`Position selected ${prediction}`)
    }

    onPositionChanged() {
        this._syncAnnotations()
    }

    drawStart = (event:MouseEvent) => {
        this._drawStartPoint = this._computePoint(event);
        this._rect = document.createElementNS('http://www.w3.org/2000/svg', 'rect')
        this._rect.setAttribute("stroke", `rgba(130, 150, 167, 1)`)
        this._rect.setAttribute("fill", `rgba(130, 150, 167, 0.65)`)
        this.viewportEl.nativeElement.appendChild(this._rect)

        this.rootEl.nativeElement.addEventListener('mousemove', this.drawMove, { passive: true });
        this.rootEl.nativeElement.addEventListener('mouseup', this.drawEnd, { passive: true });
        this._cd.markForCheck();
    };

    drawMove = (event:MouseEvent) => {
        let p = this._computePoint(event)
        let w = Math.abs(p.x - this._drawStartPoint.x);
        let h = Math.abs(p.y - this._drawStartPoint.y);
        if (p.x > this._drawStartPoint.x) {
            p.x = this._drawStartPoint.x;
        }

        if (p.y > this._drawStartPoint.y) {
            p.y = this._drawStartPoint.y;
        }

        this._rect?.setAttribute("x", `${p.x}`)
        this._rect?.setAttribute("y", `${p.y}`)
        this._rect?.setAttribute("width", `${w}`)
        this._rect?.setAttribute("height", `${h}`)

        this._cd.markForCheck();
    };

    drawEnd = ($event: MouseEvent) => {
        const minSize = 10
        if (this._rect !== undefined) {

            const width = +this._rect.getAttribute("width")!
            const height = +this._rect.getAttribute("height")!
            if (width > minSize && height > minSize) {
                this.annotations.push({
                    x0: +this._rect.getAttribute("x")!,
                    y0: +this._rect.getAttribute("y")!,
                    x1: +this._rect.getAttribute("x")! + +this._rect.getAttribute("width")!,
                    y1: +this._rect.getAttribute("y")! + +this._rect.getAttribute("height")!,
                    label: this.activeLabel,
                    table: [],
                    score: 1.0
                })
            }
        }
        this.isDrawingElement = false
        this._syncAnnotations()
        this._cd.markForCheck();
    };

    private _computePoint(event:MouseEvent) {
        let point = this.rootEl.nativeElement.createSVGPoint();
        point.x = event.clientX - this.rootEl.nativeElement.getBoundingClientRect().left;
        point.y = event.clientY - this.rootEl.nativeElement.getBoundingClientRect().top;
        point = point.matrixTransform(this.viewportEl.nativeElement.getCTM()?.inverse());
        return point;
    }

    private _setZoomable() {
        if (!this.rootEl) return;

        if (!this._zoomable) {
            this._zoomable = svgPanZoom(this.rootEl.nativeElement, {
                panEnabled: true,
                controlIconsEnabled: false,
                zoomEnabled: true,
                dblClickZoomEnabled: false,
                mouseWheelZoomEnabled: true,
                preventMouseEventsDefault: true,
                zoomScaleSensitivity: 0.2,
                minZoom: 0.5,
                maxZoom: 10,
                fit: true,
                center: true,
                onZoom: (newZoom) => {
                    this.zoomLevel = newZoom;
                    // this.zoomLevel.setValue(newZoom, { emitEvent: false });
                },
                onPan: (newPan) => {
                    // TODO: Do we need to handle pan?
                    this._cd.markForCheck();
                }
            });
        }
    }

    private _syncAnnotations() {
        this.loading = true;
        this.http
            .post<Annotation[]>(`/document/${this.documentId}/${this.pageNumber}/predictions`, this.annotations)
            .pipe(
                tap(() => console.log("Starting update")),
                takeUntil(this._onDestroy$)
            )
            .subscribe(annotations => {
                console.log("Finished update")
                this.annotations = annotations
                this.loading = false;
                this._cd.markForCheck()
            })
    }

    delete(segment: Annotation) {
        const index = this.annotations.indexOf(segment)
        if (index > -1) {
            this.annotations.splice(index, 1)
        }
        this._syncAnnotations();
    }
}
