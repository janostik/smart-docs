import {ChangeDetectionStrategy, Component, Input} from '@angular/core';
import {Annotation} from "./app-annotation-tool.component";

@Component({
    selector: 'app-annotation-cell, [app-annotation-cell]',
    standalone: true,
    imports: [],
    changeDetection: ChangeDetectionStrategy.OnPush,
    template: `
        <svg class="svg-wrapper" 
             [attr.x]="parent.x0 + segment.x0" 
             [attr.y]="parent.y0 + segment.y0" 
             [attr.width]="segment.x1 - segment.x0"
             [attr.height]="segment.y1 - segment.y0">
            
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
            fill-opacity: 0.6;
        }
        
        rect.minimal:hover {
            fill-opacity: .3;
        }
    `
})
export class AppAnnotationCellComponent {


    @Input({required: true}) parent!:Annotation;
    @Input({required: true}) segment!:Annotation;

    get fill() {
        return "#fbb03b"
    }
}
