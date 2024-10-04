import {
    AfterViewInit, ChangeDetectionStrategy,
    ChangeDetectorRef,
    Component,
    ElementRef,
    EventEmitter,
    Input,
    OnInit,
    Output,
    ViewChild
} from '@angular/core';
import {Annotation} from "./app-annotation-tool.component";

const ANNOTATION_RESIZE_BOX_SIZE = 4;

export interface AnnotationBox {
    point: DOMPoint;
    x0: number;
    y0: number;
    x1: number;
    y1: number;
}

export enum Handle {
    BR, BL, TL, TR
}

@Component({
    selector: 'app-annotation, [app-annotation]',
    standalone: true,
    imports: [],
    changeDetection: ChangeDetectionStrategy.OnPush,
    template: `
        <svg class="svg-wrapper" 
             (contextmenu)="rightClicked.emit(); $event.preventDefault()" 
             [attr.x]="offsetX + segment.x0" 
             [attr.y]="offsetY + segment.y0" 
             [attr.width]="segment.x1 - segment.x0"
             [attr.height]="segment.y1 - segment.y0"
             (dblclick)="selectIfTable()">

            @if (segment.label === 'table') {
                <g>
                    @for (cell of segment.table; track cell) {
                        <rect [attr.x]="cell.x0"
                              [attr.y]="cell.y0"
                              [attr.width]="cell.x1 - cell.x0"
                              [attr.height]="cell.y1 - cell.y0"
                              [attr.stroke]="fill"
                              style="fill-opacity: 0"
                        />
                    }
                </g>
            }
            
            <g #rect>

                <!-- Actual content -->
                <rect width="100%" height="100%"
                      class="minimal"
                      [attr.fill]="fill"
                      [attr.stroke]="fill"/>

                <rect width="100%" height="100%"
                      fill-opacity="0"
                      stroke-width="1px"
                      stroke="#FFFFFF"/>

                <!-- Resize boxes -->
                <rect #handleTL [attr.width]="resizeBoxSize" [attr.height]="resizeBoxSize"
                      [attr.x]="0" [attr.y]="0"
                      [attr.fill]="fill" cursor="nwse-resize"/>
                <rect #handleTR [attr.width]="resizeBoxSize" [attr.height]="resizeBoxSize"
                      [attr.x]="segment.x1 - segment.x0 - (resizeBoxSize)" [attr.y]="0"
                      [attr.fill]="fill"
                      cursor="nesw-resize"/>
                <rect #handleBR [attr.width]="resizeBoxSize" [attr.height]="resizeBoxSize"
                      [attr.x]="segment.x1 - segment.x0 - (resizeBoxSize)"
                      [attr.y]="segment.y1 - segment.y0 - (resizeBoxSize)"
                      [attr.fill]="fill"
                      cursor="nwse-resize"/>
                <rect #handleBL [attr.width]="resizeBoxSize" [attr.height]="resizeBoxSize"
                      [attr.x]="0" [attr.y]="segment.y1 - segment.y0 - (resizeBoxSize)"
                      [attr.fill]="fill"
                      cursor="nesw-resize"/>
            </g>

            
        </svg>
    `,
    styles: `
        svg.svg-wrapper {
            cursor: pointer;
            overflow: inherit !important;
        }
        
        svg.svg-wrapper:hover {
            filter: drop-shadow(3px 3px 2px rgba(0, 0, 0, .4));
        }

        rect {
            vector-effect: non-scaling-stroke;
        }

        rect.minimal {
            stroke-width: 0;
            fill-opacity: 0.4;
        }
        
        rect.minimal:hover {
            fill-opacity: .3;
        }
    `
})
export class AppAnnotationComponent implements OnInit, AfterViewInit {

    @Input() offsetX = 0
    @Input() offsetY = 0

    @Input({required: true}) id!: number;
    @Input({alias: "rootEl", required: true}) root!: SVGSVGElement;
    @Input({alias: "viewPortEl", required: true}) viewport!: SVGGElement;
    @Input({required: true}) segment!:Annotation;

    @Output() tableSelected = new EventEmitter<MouseEvent>();
    @Output() rightClicked = new EventEmitter<void>();
    @Output() segmentPositionChanged = new EventEmitter<{handle: Handle, start: AnnotationBox, end: Annotation}>();

    @ViewChild("rect") rect!: ElementRef<SVGRectElement>;
    @ViewChild("handleTL") handleTL!: ElementRef<SVGRectElement>;
    @ViewChild("handleTR") handleTR!: ElementRef<SVGRectElement>;
    @ViewChild("handleBR") handleBR!: ElementRef<SVGRectElement>;
    @ViewChild("handleBL") handleBL!: ElementRef<SVGRectElement>;

    isResizing = false;
    resizeBoxSize = ANNOTATION_RESIZE_BOX_SIZE;

    private _resizeHandle?: Handle;
    private _resizeStartBox?: AnnotationBox;

    constructor(private _cd: ChangeDetectorRef) {
    }

    ngOnInit() {

    }

    ngAfterViewInit(): void {

        this.handleTL?.nativeElement.addEventListener('mousedown', e => this.resizeStart(e, Handle.TL), { passive: true });
        this.handleTR?.nativeElement.addEventListener('mousedown', e => this.resizeStart(e, Handle.TR), { passive: true });
        this.handleBR?.nativeElement.addEventListener('mousedown', e => this.resizeStart(e, Handle.BR), { passive: true });
        this.handleBL?.nativeElement.addEventListener('mousedown', e => this.resizeStart(e, Handle.BL), { passive: true });
    }

    get fill() {
        // color scheme taken from: https://teenage.engineering/guides/od-11
        switch (this.segment?.label) {
            case "table":
            case "cell":
                return "#fbb03b"
            case "header":
                return "#c1272d"
            case "paragraph":
                return "#12446e"
            default:
                return "#ff6600"
        }
    }

    resizeStart = (event:MouseEvent, handle:Handle) => {
        this._resizeHandle = handle;
        this._resizeStartBox = {
            point: this._computePoint(event),
            x0: this.segment.x0,
            y0: this.segment.y0,
            x1: this.segment.x1,
            y1: this.segment.y1,
        };

        // now we attach mousemove and end move events to the main SVG:
        this.root.addEventListener('mousemove', this.resizeMove, { passive: true });
        this.root.addEventListener('mouseup', this.resizeEnd, { passive: true });
        event.stopPropagation();
        this._cd.markForCheck();
    };

    resizeMove = (event:MouseEvent) => {
        this.isResizing = true;

        let current = this._computePoint(event);
        const start = this._resizeStartBox;
        if (start) {
            let diff = {
                x: current.x - start.point.x,
                y: current.y - start.point.y,
            };

            // const minBox = (n: number) => n < (this.resizeBoxSize * 2) ? (this.resizeBoxSize * 2) : n;

            const changedPosition = { ...this.segment };
            switch (this._resizeHandle) {
                case Handle.BR:
                    changedPosition.x1 = start.x1 + diff.x;
                    changedPosition.y1 = start.y1 + diff.y;
                    // changedPosition.width = minBox(start.width + diff.x);
                    // changedPosition.height = minBox(start.height + diff.y);
                    break;
                case Handle.BL:
                    changedPosition.x0 = start.x0 + diff.x;
                    changedPosition.y1 = start.y1 + diff.y;
                    // changedPosition.width = minBox(start.width - diff.x);
                    // changedPosition.height = minBox(start.height + diff.y);
                    break;
                case Handle.TL:
                    changedPosition.x0 = start.x0 + diff.x;
                    changedPosition.y0 = start.y0 + diff.y;
                    // changedPosition.width = minBox(start.width - diff.x);
                    // changedPosition.height = minBox(start.height - diff.y);
                    break;
                case Handle.TR:
                    changedPosition.y0 = start.y0 + diff.y;
                    changedPosition.x1 = start.x1 + diff.x;
                    // changedPosition.width = minBox(start.width + diff.x);
                    // changedPosition.height = minBox(start.height - diff.y);
                    break;
            }
            // TODO: Clamp to min/max width
            this.segment.x0 = changedPosition.x0
            this.segment.x1 = changedPosition.x1
            this.segment.y0 = changedPosition.y0
            this.segment.y1 = changedPosition.y1
        }

        event.stopPropagation();
        this._cd.markForCheck();
    };

    resizeEnd = () => {
        this.isResizing = false;
        this.root.removeEventListener('mousemove', this.resizeMove);
        this.root.removeEventListener('mouseup', this.resizeEnd);
        this.segmentPositionChanged.emit({
            handle: this._resizeHandle!,
            start: this._resizeStartBox!,
            end: this.segment
        });
        this._cd.markForCheck();
    };

    private _computePoint(event:MouseEvent) {
        let point = this.root.createSVGPoint();
        if (point) {
            point.x = event.clientX;
            point.y = event.clientY;
            point = point.matrixTransform(this.viewport.getCTM()?.inverse());
        }
        return point;
    }

    selectIfTable() {
        if (this.segment?.label === 'table') {
            console.log("Selecting table...")
            this.tableSelected.emit()
        }
    }
}
