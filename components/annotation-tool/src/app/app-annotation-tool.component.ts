import {ChangeDetectorRef, Component, ElementRef, inject, OnInit, ViewChild} from '@angular/core';
import {HttpClient} from "@angular/common/http";
import {Subject, takeUntil, tap} from "rxjs";
import {AppAnnotationComponent} from "./app-annotation.component";

import svgPanZoom from "svg-pan-zoom";
import Instance = SvgPanZoom.Instance;
import {AppAnnotationCellComponent} from "./app-annotation-cell.component";

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
        AppAnnotationCellComponent,
    ],
    template: `
        @if (imageUrl) {
            <svg #root>
                <g #viewport>
                    @if (selectedTable) {
                        <defs>
                            <clipPath id="clip">
                                <rect [attr.x]="selectedTable.x0"
                                      [attr.y]="selectedTable.y0"
                                      [attr.width]="selectedTable.x1 - selectedTable.x0"
                                      [attr.height]="selectedTable.y1 - selectedTable.y0"
                                ></rect>
                            </clipPath>
                        </defs>
                        <image x="0" y="0"
                               [attr.width]="width"
                               [attr.height]="height"
                               preserveAspectRatio="xMidYMax meet"
                               [attr.href]="imageUrl"
                               clip-path="url(#clip)"></image>

                        @for (segment of selectedTable.table; track segment; let index = $index) {
                            <svg:g app-annotation-cell [id]="index"
                                   [attr.id]="'el-' + index"
                                   [segment]="segment"
                                   [parent]="selectedTable"
                            />
                        }
                    } @else {
                        <image x="0" y="0"
                               [attr.href]="imageUrl"
                               [attr.width]="width"
                               [attr.height]="height"
                               preserveAspectRatio="xMidYMax meet"/>
                        @for (segment of annotations; track segment; let index = $index) {
                            <svg:g app-annotation [id]="index"
                                   [attr.id]="'el-' + index"
                                   [segment]="segment"
                                   [rootEl]="rootEl.nativeElement"
                                   [viewPortEl]="viewportEl.nativeElement"
                                   (rightClicked)="delete(segment)"
                                   (tableSelected)="selectedTable = segment"
                                   (segmentPositionChanged)="onPositionChanged()"
                            />
                        }
                    }
                </g>
            </svg>

            <div class="floating">
                @if (selectedTable) {
                    @if (activeTool) {
                        <button (click)="activeTool = undefined">Cancel</button>
                    } @else {
                        <button (click)="activeTool = 'MERGE'">Join cells</button>
                        <button (click)="activeTool = 'SPLIT_COLS'">Split cols</button>
                        <button (click)="activeTool = 'SPLIT_ROWS'">Split rows</button>
                    }
                    
                    <button (click)="selectedTable = undefined">Exit</button>
                } @else {
                    @if (activeTool) {
                        <button (click)="activeTool = undefined">(C)ancel</button>
                    } @else {
                        <button (click)="activeTool = 'DRAW'">(C)reate</button>
                    }
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

    width = "595px"
    height = "843px"

    type: ('DOCUMENT' | 'TABLE') = 'DOCUMENT';
    documentId: string = '';
    pageNumber: string = '';
    imageUrl?: string
    annotations: Annotation[] = [];
    selectedTable?:Annotation

    @ViewChild("root") rootEl!: ElementRef<SVGSVGElement>;
    @ViewChild("viewport") viewportEl!: ElementRef<SVGGElement>;

    http = inject(HttpClient)
    loading = false;
    zoomLevel: number = 1;
    activeLabel = "paragraph"

    private _zoomable?: Instance;
    private _rect?: SVGRectElement;
    private _line?: SVGLineElement;
    // TODO: Rewrite to active tool enum
    private _activeTool?: ('DRAW' | 'SPLIT_ROWS' | 'SPLIT_COLS' | 'MERGE')

    private _drawStartPoint: { x: number; y: number } = {x: 0, y: 0};
    protected _onDestroy$ = new Subject<void>();

    set activeTool(value) {
        this._activeTool = value;
        this._clearFragments();

        if (this._activeTool) {
            this._zoomable?.disablePan()

            switch (this._activeTool) {
                case "DRAW":
                    this.rootEl.nativeElement.removeEventListener('mousedown', this.drawRectToolStart);
                    this.rootEl.nativeElement.removeEventListener('mousemove', this.drawRectToolMove);
                    this.rootEl.nativeElement.removeEventListener('mouseup', this.drawRectToolEnd);
                    break
                case "MERGE":
                    this.rootEl.nativeElement.addEventListener('mousedown', this.drawRectToolStart, { passive: true });
                    this.rootEl.nativeElement.addEventListener('mousemove', this.drawRectToolMove, { passive: true });
                    this.rootEl.nativeElement.addEventListener('mouseup', this.drawRectToolEnd, { passive: true });
                    break
                case "SPLIT_COLS":
                case "SPLIT_ROWS":
                    this.rootEl.nativeElement.addEventListener('mousedown', this.splitCells, { passive: true });
                    this.rootEl.nativeElement.addEventListener('mousemove', this.drawLineToolMove, { passive: true });

            }
        } else {
            this._zoomable?.enablePan()

            this.rootEl.nativeElement.removeEventListener('mousedown', this.splitCells);
            this.rootEl.nativeElement.removeEventListener('mousedown', this.drawRectToolStart);
            this.rootEl.nativeElement.removeEventListener('mousemove', this.drawRectToolMove);
            this.rootEl.nativeElement.removeEventListener('mouseup', this.drawRectToolEnd);
            this.rootEl.nativeElement.removeEventListener('mouseup', this.drawLineToolMove);
        }
        this._cd.markForCheck();
    }

    get activeTool() {
        return this._activeTool;
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

    onPositionChanged() {
        this._syncAnnotations()
    }

    drawRectToolStart = (event:MouseEvent) => {
        this._drawStartPoint = this._computePoint(event);
        this._rect = document.createElementNS('http://www.w3.org/2000/svg', 'rect')
        this._rect.setAttribute("stroke", `rgba(130, 150, 167, 1)`)
        this._rect.setAttribute("fill", `rgba(130, 150, 167, 0.65)`)
        this.viewportEl.nativeElement.appendChild(this._rect)
        this._cd.markForCheck();
    };

    drawRectToolMove = (event:MouseEvent) => {
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

    drawRectToolEnd = ($event: MouseEvent) => {
        const minSize = 10
        if (this._rect !== undefined) {

            const width = +this._rect.getAttribute("width")!
            const height = +this._rect.getAttribute("height")!
            if (width > minSize && height > minSize) {
                switch (this.activeTool) {
                    case "DRAW":
                        this.annotations.push({
                            x0: +this._rect.getAttribute("x")!,
                            y0: +this._rect.getAttribute("y")!,
                            x1: +this._rect.getAttribute("x")! + +this._rect.getAttribute("width")!,
                            y1: +this._rect.getAttribute("y")! + +this._rect.getAttribute("height")!,
                            label: this.activeLabel,
                            table: [],
                            score: 1.0
                        })
                        break;
                    case "MERGE":
                        let x0 = +this._rect.getAttribute("x")!
                        let y0 = +this._rect.getAttribute("y")!
                        let x1 = +this._rect.getAttribute("x")! + +this._rect.getAttribute("width")!
                        let y1 = +this._rect.getAttribute("y")! + +this._rect.getAttribute("height")!

                        let toMerge:Annotation[] = []
                        for (let el of this.selectedTable!.table) {
                            if (this.overlaps(el, x0, y0, x1, y1)) {
                                toMerge.push(el)
                            }
                        }
                        let minX = 99999
                        let minY = 99999
                        let maxX = -99999
                        let maxY = -99999
                        for (let el of toMerge) {
                            minX = Math.min(el.x0, minX)
                            minY = Math.min(el.y0, minY)
                            maxX = Math.max(el.x1, maxX)
                            maxY = Math.max(el.y1, maxY)

                            this.selectedTable?.table.splice(this.selectedTable?.table.indexOf(el), 1)
                        }
                        this.selectedTable?.table.push({
                            x0: minX,
                            y0: minY,
                            x1: maxX,
                            y1: maxY,
                            label: "cell",
                            table: [],
                            score: 1.0
                        })
                }
            }
        }
        this.activeTool = undefined
        this._syncAnnotations()
        this._cd.markForCheck();
    };

    splitCells = (event:MouseEvent) => {
        let p = this._computePoint(event)

        let overlaps:(ann: Annotation, p: DOMPoint) => boolean;
        let split:(ann: Annotation, p: DOMPoint) => [Annotation, Annotation];

        switch (this._activeTool) {
            case "SPLIT_ROWS":
                overlaps = (ann: Annotation, p: DOMPoint) => {
                    return ann.y0 + this.selectedTable!.y0 < p.y && ann.y1 + this.selectedTable!.y0 > p.y;
                }
                split = (ann: Annotation, p: DOMPoint):[Annotation, Annotation] => {
                    return [
                        {...ann, y1: p.y - this.selectedTable!.y0},
                        {...ann, y0: p.y - this.selectedTable!.y0},
                    ];
                }
                break
            case "SPLIT_COLS":
                overlaps = (ann: Annotation, p: DOMPoint) => {
                    return ann.x0 + this.selectedTable!.x0 < p.x && ann.x1 + this.selectedTable!.x0 > p.x;
                }
                split = (ann: Annotation, p: DOMPoint):[Annotation, Annotation] => {
                    return [
                        {...ann, x1: p.x - this.selectedTable!.x0},
                        {...ann, x0: p.x - this.selectedTable!.x0},
                    ];
                }
                break
            default:
                return
        }

        const toSplit:Annotation[] = []
        for (let ann of this.selectedTable!.table) {
            if (overlaps(ann, p)) {
                toSplit.push(ann)
            }
        }

        for (let ann of toSplit) {
            const splitCells = split(ann, p)
            this.selectedTable!.table.splice(this.selectedTable!.table.indexOf(ann), 1)
            this.selectedTable!.table.push(...splitCells)
        }

        this._syncAnnotations()
        this._cd.markForCheck();
    };

    drawLineToolMove = (event:MouseEvent) => {
        let p = this._computePoint(event)
        if (!this._line) {
            this._line = document.createElementNS('http://www.w3.org/2000/svg', 'line')
            this._line.setAttribute("stroke", `rgba(130, 150, 167, 1)`)
            this._line.setAttribute("fill", `rgba(130, 150, 167, 0.65)`)
            this.viewportEl.nativeElement.appendChild(this._line)
        }
        switch (this._activeTool) {
            case "SPLIT_ROWS":
                this._line?.setAttribute("x1", "0")
                this._line?.setAttribute("x2", `${this.width}`)
                this._line?.setAttribute("y1", `${p.y}`)
                this._line?.setAttribute("y2", `${p.y}`)
                break
            case "SPLIT_COLS":
                this._line?.setAttribute("x1", `${p.x}`)
                this._line?.setAttribute("x2", `${p.x}`)
                this._line?.setAttribute("y1", "0")
                this._line?.setAttribute("y2", `${this.height}`)
                break
        }
        this._line?.setAttribute("stroke-width", "1")
        this._line?.setAttribute("stroke", "black")

        // <line x1="0" y1="80" x2="100" y2="20" stroke-width="15" stroke="black"></line>


        this._cd.markForCheck();
    };

    private _clearFragments() {
        if (this._rect) {
            this._rect.remove()
            this._rect = undefined
        }
        if (this._line) {
            this._line.remove()
            this._line = undefined
        }
    }

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
                takeUntil(this._onDestroy$)
            )
            .subscribe(annotations => {
                this.annotations = annotations
                if (this.selectedTable) {
                    // We need to lookup the table again since after updating annotations, it's different set of objects.
                    this.selectedTable = this.annotations.find(a => a.x0 === this.selectedTable!.x0
                        && a.x1 === this.selectedTable!.x1
                        && a.y0 === this.selectedTable!.y0
                        && a.y1 === this.selectedTable!.y1
                    )
                }
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

    private overlaps(annotation: Annotation, x0: number, y0: number, x1: number, y1: number): boolean {
        const xOverlap = Math.max(0, Math.min(annotation.x1 + this.selectedTable!.x0, x1) - Math.max(annotation.x0 + this.selectedTable!.x0, x0))
        const yOverlap = Math.max(0, Math.min(annotation.y1 + this.selectedTable!.y0, y1) - Math.max(annotation.y0 + this.selectedTable!.y0, y0))
        return (xOverlap * yOverlap) > 0
    }
}
