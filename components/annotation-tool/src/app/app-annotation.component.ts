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

interface AnnotationBox {
    point: DOMPoint;
    x0: number;
    y0: number;
    x1: number;
    y1: number;
}

enum Handle {
    BR, BL, TL, TR
}

@Component({
    selector: 'app-annotation, [app-annotation]',
    standalone: true,
    imports: [],
    changeDetection: ChangeDetectionStrategy.OnPush,
    template: `
        <svg class="svg-wrapper" [attr.x]="box.x0" [attr.y]="box.y0" [attr.width]="box.x1 - box.x0" [attr.height]="box.y1 - box.y0" (contextmenu)="rightClicked.emit(); $event.preventDefault()">

            <g #rect>
                
                <!-- Actual content -->
                <rect width="100%" height="100%"
                      class="minimal"
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
                      [attr.x]="box.x1 - box.x0 - (resizeBoxSize)" [attr.y]="0"
                      [attr.fill]="fill"
                      cursor="nesw-resize"/>
                <rect #handleBR [attr.width]="resizeBoxSize" [attr.height]="resizeBoxSize"
                      [attr.x]="box.x1 - box.x0 - (resizeBoxSize)"
                      [attr.y]="box.y1 - box.y0 - (resizeBoxSize)"
                      [attr.fill]="fill"
                      cursor="nwse-resize"/>
                <rect #handleBL [attr.width]="resizeBoxSize" [attr.height]="resizeBoxSize"
                      [attr.x]="0" [attr.y]="box.y1 - box.y0 - (resizeBoxSize)"
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
            fill: #ff6600;
            fill-opacity: 0.6;
        }
        
        rect.minimal:hover {
            fill-opacity: .3;
        }
    `
})
export class AppAnnotationComponent implements OnInit, AfterViewInit {

    @Input({required: true}) id!: number;
    @Input({required: true}) fill = "rgba(0,0,0,0)";
    @Input() headerWidth = 0;
    @Input() stroke = "#ff6600";
    @Input({alias: "rootEl", required: true}) root!: SVGSVGElement;
    @Input({alias: "viewPortEl", required: true}) viewport!: SVGGElement;
    @Input({required: true}) box!:Annotation;

    @Output() selected = new EventEmitter<MouseEvent>();
    @Output() rightClicked = new EventEmitter<void>();
    @Output() segmentPositionChanged = new EventEmitter<void>();

    @ViewChild("rect") rect!: ElementRef<SVGRectElement>;
    @ViewChild("handleTL") handleTL!: ElementRef<SVGRectElement>;
    @ViewChild("handleTR") handleTR!: ElementRef<SVGRectElement>;
    @ViewChild("handleBR") handleBR!: ElementRef<SVGRectElement>;
    @ViewChild("handleBL") handleBL!: ElementRef<SVGRectElement>;

    isResizing = false;
    resizeBoxSize = ANNOTATION_RESIZE_BOX_SIZE;

    private _resizeHandle?: Handle;
    private _resizeStartBox?: AnnotationBox;

    private _selectEventFired = false;
    private _selectSegmentTimeout?: any;

    constructor(private _cd: ChangeDetectorRef) {
    }

    ngOnInit() {

    }

    ngAfterViewInit(): void {

        // Remove all eventListeners on rect element
        this.rect.nativeElement.removeEventListener('click', this.select);

        this.handleTL?.nativeElement.addEventListener('mousedown', e => this.resizeStart(e, Handle.TL), { passive: true });
        this.handleTR?.nativeElement.addEventListener('mousedown', e => this.resizeStart(e, Handle.TR), { passive: true });
        this.handleBR?.nativeElement.addEventListener('mousedown', e => this.resizeStart(e, Handle.BR), { passive: true });
        this.handleBL?.nativeElement.addEventListener('mousedown', e => this.resizeStart(e, Handle.BL), { passive: true });
    }

    resizeStart = (event:MouseEvent, handle:Handle) => {
        this._resizeHandle = handle;
        this._resizeStartBox = {
            point: this._computePoint(event),
            x0: this.box.x0,
            y0: this.box.y0,
            x1: this.box.x1,
            y1: this.box.y1,
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

            const changedPosition = { ...this.box };
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
            this.box.x0 = changedPosition.x0
            this.box.x1 = changedPosition.x1
            this.box.y0 = changedPosition.y0
            this.box.y1 = changedPosition.y1
        }

        event.stopPropagation();
        this._cd.markForCheck();
    };

    resizeEnd = () => {
        this.isResizing = false;
        this.root.removeEventListener('mousemove', this.resizeMove);
        this.root.removeEventListener('mouseup', this.resizeEnd);
        this.segmentPositionChanged.emit();
        this._cd.markForCheck();
    };

    select = (event:MouseEvent) => {
        if (!this._selectEventFired) {
            this.selected.emit(event);
            // To prevent emitting selected event twice
            this._setSelectEventFired();
        }

    }

    private _computePoint(event:MouseEvent) {
        let point = this.root.createSVGPoint();
        if (point) {
            point.x = event.clientX;
            point.y = event.clientY;
            point = point.matrixTransform(this.viewport.getCTM()?.inverse());
        }
        return point;
    }

    // To prevent emitting selected event twice
    private _setSelectEventFired() {
        // set eventFired to TRUE...
        this._selectEventFired = true;
        clearTimeout(this._selectSegmentTimeout);
        this._selectSegmentTimeout = setTimeout(() => {
            // ... and clear it soon so it won't hang with TRUE status (we cannot rely on clearing in methods dragEnd and select because these methods are not called always together)
            this._selectEventFired = false;
            this._cd.markForCheck();
        }, 10);
    }
}
