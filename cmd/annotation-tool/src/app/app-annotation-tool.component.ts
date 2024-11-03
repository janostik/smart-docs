import {ChangeDetectorRef, Component, ElementRef, HostListener, inject, OnInit, ViewChild} from '@angular/core';
import {HttpClient} from "@angular/common/http";
import {Subject, takeUntil} from "rxjs";
import {AnnotationBox, AppAnnotationComponent, Handle} from "./app-annotation.component";

import svgPanZoom from "svg-pan-zoom";
import Instance = SvgPanZoom.Instance;
import {TooltipDirective} from "./app-tooltip.directive";

export interface Annotation {
    score: number,
    label: string,
    x0: number,
    x1: number,
    y0: number,
    y1: number,
    table: Annotation[]
}

const MIN_SIZE = 5;

@Component({
    selector: 'app-annotation-tool',
    standalone: true,
    imports: [
        AppAnnotationComponent,
        TooltipDirective,
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
                            <svg:g app-annotation [id]="index"
                                   [attr.id]="'el-' + index"
                                   [offsetX]="selectedTable.x0"
                                   [offsetY]="selectedTable.y0"
                                   [sizeLimitX]="selectedTable.x1 - selectedTable.x0"
                                   [sizeLimitY]="selectedTable.y1 - selectedTable.y0"
                                   [segment]="segment"
                                   [rootEl]="rootEl.nativeElement"
                                   [viewPortEl]="viewportEl.nativeElement"
                                   (mouseover)="highlightedSegment = segment"
                                   (mouseout)="highlightedSegment = undefined"
                                   (rightClicked)="deleteCell(segment)"
                                   (segmentPositionChanged)="onPositionChanged($event)"
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
            
            @if (selectedTable) {
                <div class="btn-row">

                    <button (click)="selectedTable = undefined" appTooltip="Stop Annotating Tables">
                        <img src="/assets/icons/editor/exit.svg" style="transform: rotate(180deg)" alt="Stop Annotating Tables">
                        <span>(ESC)</span>
                    </button>
                    
                    <div class="delimiter"></div>
                    
                    <button [class.active]="activeTool == 'SPLIT_COLS'" appTooltip="Split Columns" (click)="activeTool ? activeTool = undefined : activeTool = 'SPLIT_COLS'">
                        <img src="/assets/icons/editor/table-columns.svg" alt="Split Columns">
                        @if (activeTool && activeTool == 'SPLIT_COLS') {
                            <span>(ESC)</span>
                        } @else {
                            <span>(1)</span>
                        }
                    </button>
                    <button [class.active]="activeTool == 'SPLIT_ROWS'" appTooltip="Split Rows" (click)="activeTool ? activeTool = undefined : activeTool = 'SPLIT_ROWS'">
                        <img src="/assets/icons/editor/table-rows.svg" alt="Split Rows">
                        @if (activeTool && activeTool == 'SPLIT_ROWS') {
                            <span>(ESC)</span>
                        } @else {
                            <span>(2)</span>
                        }
                    </button>
                    <button [class.active]="activeTool == 'MERGE'" appTooltip="Join Cells" (click)="activeTool ? activeTool = undefined : activeTool = 'MERGE'">
                        <img src="/assets/icons/editor/object-union.svg" alt="Join Cells">
                        @if (activeTool && activeTool == 'MERGE') {
                            <span>(ESC)</span>
                        } @else {
                            <span>(3)</span>
                        }
                    </button>
                    <button (click)="createTable()" appTooltip="Reset and Create New" >
                        <img src="/assets/icons/editor/square-plus.svg" alt="Reset and Create New">
                        <span>(R)</span>
                    </button>
                </div>
                
                
            } @else {
                <div class="btn-row">
                    <button [class.active]="activeTool == 'DRAW_P'" appTooltip="Draw Paragraph" (click)="activeTool ? activeTool = undefined : activeTool = 'DRAW_P'">
                        <img src="/assets/icons/editor/paragraph.svg" alt="Draw Paragraph">
                        @if (activeTool && activeTool == 'DRAW_P') {
                            <span>(ESC)</span>
                        } @else {
                            <span>(1)</span>
                        }
                    </button>
                    <button [class.active]="activeTool == 'DRAW_HEADER'" appTooltip="Draw Header" (click)="activeTool ? activeTool = undefined : activeTool = 'DRAW_HEADER'">
                        <img src="/assets/icons/editor/heading.svg" alt="Draw Header">
                        @if (activeTool && activeTool == 'DRAW_HEADER') {
                            <span>(ESC)</span>
                        } @else {
                            <span>(2)</span>
                        }
                    </button>
                    <button [class.active]="activeTool == 'DRAW_TABLE'" appTooltip="Draw Table" (click)="activeTool ? activeTool = undefined : activeTool = 'DRAW_TABLE'">
                        <img src="/assets/icons/editor/table.svg" alt="Draw Table">
                        @if (activeTool && activeTool == 'DRAW_TABLE') {
                            <span>(ESC)</span>
                        } @else {
                            <span>(3)</span>
                        }
                    </button>
                    <button [class.active]="activeTool == 'DRAW_IMAGE'" appTooltip="Draw Image" (click)="activeTool ? activeTool = undefined : activeTool = 'DRAW_IMAGE'">
                        <img src="/assets/icons/editor/image.svg" alt="Draw Image">
                        @if (activeTool && activeTool == 'DRAW_IMAGE') {
                            <span>(ESC)</span>
                        } @else {
                            <span>(4)</span>
                        }
                    </button>
                </div>
            }
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

        .btn-row {
            position: fixed;
            left: 16px; 
            top: 50%;
            transform: translateY(-50%);
            display: flex;
            flex-direction: column;
            gap: 8px;
            background-color: rgba(255, 255, 255, 0.9); 
            padding: 8px;
            border-radius: 8px;
            box-shadow: 0 4px 10px rgba(0, 0, 0, 0.1);
            z-index: 100; 
        }

        .btn-row button {
            position: relative;
            border: none;
            background: transparent;
            cursor: pointer;
            padding: 8px;
            display: flex;
            flex-direction: column;
            align-items: center;
            justify-content: center;
        }

        .btn-row button.active {
            background-color: rgba(0, 123, 255, 0.1); /* Light blue background when active */
            border-radius: 4px;
        }
        
        .btn-row .delimiter {
            border-bottom: 1px solid #777;
        }

        .btn-row button img {
            height: 18px;
            width: auto;
        }
        
        .btn-row button span {
            position: absolute;
            top: 0;
            right: 0;
            font-size: 9px;
            color: #777;
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
    history: Annotation[][] = []
    selectedTable?:Annotation

    @ViewChild("root") rootEl!: ElementRef<SVGSVGElement>;
    @ViewChild("viewport") viewportEl!: ElementRef<SVGGElement>;

    http = inject(HttpClient)
    loading = false;
    zoomLevel: number = 1;
    highlightedSegment?: Annotation;

    private _zoomable?: Instance;
    private _rect?: SVGRectElement;
    private _line?: SVGLineElement;

    private _activeTool?: ('DRAW_P' | 'DRAW_HEADER' | 'DRAW_TABLE' | 'DRAW_IMAGE' | 'SPLIT_ROWS' | 'SPLIT_COLS' | 'MERGE')
    private _drawStartPoint: { x: number; y: number } = {x: 0, y: 0};
    private _onDestroy$ = new Subject<void>();
    private _shiftPressed: boolean = false;

    @HostListener('document:keyup', ['$event'])
    handleKeyUp(event: KeyboardEvent) {
        if (!event.shiftKey) {
            this._shiftPressed = false;
        }
    }

    @HostListener('document:keydown', ['$event'])
    handleKeyDown(event: KeyboardEvent) {
        if (event.shiftKey) {
            this._shiftPressed = true;
        }

        if (event.key === 'Escape' && this.activeTool) {
            event.stopImmediatePropagation();
            this.activeTool = undefined
            return
        }

        if (event.key === 'Escape' && this.selectedTable) {
            event.stopImmediatePropagation();
            this.selectedTable = undefined
            return
        }

        // if (event.key === 'z' && (event.ctrlKey || event.metaKey)) {
        if (event.key === 'z') {
            // handle undo action
            // first item on stack is current state
            this.history.pop()
            const previous = this.history.pop()
            if (previous) {
                this.annotations = previous
                this._syncAnnotations()
            }
            event.stopImmediatePropagation()
        }

        if (this.selectedTable) {
            // Handling table drawing keys
            if (event.key === 'r' && !event.metaKey) {
                event.stopPropagation()
                this.createTable()
            } else if (event.key === '1') {
                event.stopImmediatePropagation();
                this.activeTool = 'SPLIT_COLS'
            } else  if (event.key === '2') {
                event.stopImmediatePropagation();
                this.activeTool = 'SPLIT_ROWS'
            } else if (event.key === '3') {
                event.stopImmediatePropagation();
                this.activeTool = 'MERGE'
            }
        } else {
            // Handling element drawing keys
            if (event.key === '1') {
                event.stopImmediatePropagation();
                this.activeTool = 'DRAW_P'
            } else  if (event.key === '2') {
                event.stopImmediatePropagation();
                this.activeTool = 'DRAW_HEADER'
            } else if (event.key === '3') {
                event.stopImmediatePropagation();
                this.activeTool = 'DRAW_TABLE'
            } else if (event.key === '4') {
                event.stopImmediatePropagation();
                this.activeTool = 'DRAW_IMAGE'
            }
        }
    }

    get cleanedAnnotations() {
        return this.annotations
            .filter(a => {
                if (!this._isValid(a)) {
                    return false
                }
                if (a.table?.length > 0) {
                    a.table = a.table.filter(a => this._isValid(a))
                }
                return true
            })
            .map(a => {
                if (a.table?.length > 0) {
                    a.table = a.table.map(t => ({...t, label: "cell"}))
                }
                return a
            })
    }

    private _isValid(a: Annotation) {
        if (a.x0 > a.x1 || a.y0 > a.y1) {
            // Invalid shape
            return false
        }
        if (a.x1 - a.x0 < MIN_SIZE || a.y1 - a.y0 < MIN_SIZE) {
            // too small
            return false
        }
        return true
    }

    set activeTool(value) {
        this._activeTool = value;
        this._clearFragments();

        if (this._activeTool) {
            this._zoomable?.disablePan()

            switch (this._activeTool) {
                case "DRAW_P":
                case "DRAW_HEADER":
                case "DRAW_TABLE":
                case "DRAW_IMAGE":
                    this.rootEl.nativeElement.addEventListener('mousedown', this.drawRectToolStart, { passive: true });
                    this.rootEl.nativeElement.addEventListener('mousemove', this.drawRectToolMove, { passive: true });
                    this.rootEl.nativeElement.addEventListener('mouseup', this.drawRectToolEnd, { passive: true });
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

    get activeLabel() {
        switch (this.activeTool) {
            case "DRAW_P":
                return "paragraph";
            case "DRAW_TABLE":
                return "table";
            case "DRAW_HEADER":
                return "header";
            case "DRAW_IMAGE":
                return "illustration";
            default:
                return "other"
        }
    }

    constructor(private elementRef: ElementRef, private _cd: ChangeDetectorRef) {
        this.type = this.elementRef.nativeElement.getAttribute('type');
        this.documentId = this.elementRef.nativeElement.getAttribute('document-id');
        this.pageNumber = this.elementRef.nativeElement.getAttribute('page-number');
        this.width = this.elementRef.nativeElement.getAttribute('image-width');
        this.height = this.elementRef.nativeElement.getAttribute('image-height');
        this.imageUrl = `/images/${this.documentId}/${this.pageNumber}.jpg`
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

    onPositionChanged(event?: {handle: Handle, start: AnnotationBox, end: Annotation}) {
        if (event && this._shiftPressed) {
            let xStart, yStart, xEnd, yEnd;
            switch (event.handle) {
                case Handle.TL:
                    xStart = event.start.x0
                    xEnd = event.end.x0
                    yStart = event.start.y0
                    yEnd = event.end.y0
                    break
                case Handle.TR:
                    xStart = event.start.x1
                    xEnd = event.end.x1
                    yStart = event.start.y0
                    yEnd = event.end.y0
                    break
                case Handle.BL:
                    xStart = event.start.x0
                    xEnd = event.end.x0
                    yStart = event.start.y1
                    yEnd = event.end.y1
                    break
                case Handle.BR:
                    xStart = event.start.x1
                    xEnd = event.end.x1
                    yStart = event.start.y1
                    yEnd = event.end.y1
                    break
            }
            for (let ann of this.selectedTable!.table) {
                if (ann.x0 == xStart) {
                    ann.x0 = xEnd
                }
                if (ann.x1 == xStart) {
                    ann.x1 = xEnd
                }
                if (ann.y0 == yStart) {
                    ann.y0 = yEnd
                }
                if (ann.y1 == yStart) {
                    ann.y1 = yEnd
                }
            }
        }
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
        if (this._rect !== undefined) {

            const width = +this._rect.getAttribute("width")!
            const height = +this._rect.getAttribute("height")!
            if (width > MIN_SIZE && height > MIN_SIZE) {
                switch (this.activeTool) {
                    case "DRAW_P":
                    case "DRAW_HEADER":
                    case "DRAW_TABLE":
                    case "DRAW_IMAGE":
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
                            if (this._overlaps(el, x0, y0, x1, y1)) {
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
        this._rect?.remove()
        this._rect = undefined;
        this._syncAnnotations()
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
        if (this._shiftPressed) {
            for (let ann of this.selectedTable!.table) {
                if (overlaps(ann, p)) {
                    toSplit.push(ann)
                }
            }
        } else {
            toSplit.push(this.highlightedSegment!)
        }

        for (let ann of toSplit) {
            const splitCells = split(ann, p)
            this.selectedTable!.table.splice(this.selectedTable!.table.indexOf(ann), 1)
            this.selectedTable!.table.push(...splitCells)
        }

        this._syncAnnotations()
    };

    drawLineToolMove = (event:MouseEvent) => {
        let p = this._computePoint(event)
        if (!this._line) {
            this._line = document.createElementNS('http://www.w3.org/2000/svg', 'line')
            this._line.setAttribute("stroke", `rgba(130, 150, 167, 1)`)
            this._line.setAttribute("fill", `rgba(130, 150, 167, 0.65)`)
            this._line.style.pointerEvents = "none"
            this.viewportEl.nativeElement.appendChild(this._line)
        }
        const hoveredSegment = this.highlightedSegment;
        switch (this._activeTool) {
            case "SPLIT_ROWS":
                this._line?.setAttribute("x1", !this._shiftPressed && !!hoveredSegment ? `${this.selectedTable!.x0 + hoveredSegment.x0}` : "0")
                this._line?.setAttribute("x2", !this._shiftPressed && !!hoveredSegment ? `${this.selectedTable!.x0 + hoveredSegment.x1}` : `${this.width}`)
                this._line?.setAttribute("y1", `${p.y}`)
                this._line?.setAttribute("y2", `${p.y}`)
                break
            case "SPLIT_COLS":
                this._line?.setAttribute("x1", `${p.x}`)
                this._line?.setAttribute("x2", `${p.x}`)
                this._line?.setAttribute("y1", !this._shiftPressed && !!hoveredSegment ? `${this.selectedTable!.y0 + hoveredSegment.y0}` : "0")
                this._line?.setAttribute("y2", !this._shiftPressed && !!hoveredSegment ? `${this.selectedTable!.y0 + hoveredSegment.y1}` : `${this.height}`)
                break
        }
        this._line?.setAttribute("stroke-width", "1")
        this._line?.setAttribute("stroke-dasharray", "0 4 0")
        this._line?.setAttribute("stroke", "black")

        this._cd.markForCheck();
    };

    createTable() {
        this.selectedTable!.table = [{
            x0: 0,
            x1: this.selectedTable!.x1 - this.selectedTable!.x0,
            y0: 0,
            y1: this.selectedTable!.y1 - this.selectedTable!.y0,
            table: [],
            score: 1.0,
            label: 'cell'
        }]
        this._syncAnnotations()
    }

    delete(segment: Annotation) {
        const index = this.annotations.indexOf(segment)
        if (index > -1) {
            this.annotations.splice(index, 1)
        }
        this._syncAnnotations();
    }

    deleteCell(segment: Annotation) {
        const index = this.selectedTable!.table.indexOf(segment)
        if (index > -1) {
            this.selectedTable?.table.splice(index, 1)
        }
        this._syncAnnotations();
    }

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
            .post<Annotation[]>(`/document/${this.documentId}/${this.pageNumber}/predictions`, this.cleanedAnnotations)
            .pipe(
                takeUntil(this._onDestroy$)
            )
            .subscribe(annotations => {
                this.annotations = annotations
                this.history.push(this.annotations)
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

    private _overlaps(annotation: Annotation, x0: number, y0: number, x1: number, y1: number): boolean {
        const xOverlap = Math.max(0, Math.min(annotation.x1 + this.selectedTable!.x0, x1) - Math.max(annotation.x0 + this.selectedTable!.x0, x0))
        const yOverlap = Math.max(0, Math.min(annotation.y1 + this.selectedTable!.y0, y1) - Math.max(annotation.y0 + this.selectedTable!.y0, y0))
        return (xOverlap * yOverlap) > 0
    }
}
