import { Directive, ElementRef, HostListener, Input, Renderer2 } from '@angular/core';

@Directive({
    selector: '[appTooltip]',
    standalone: true
})
export class TooltipDirective {
    @Input('appTooltip') tooltipText: string = '';  // Tooltip text passed from the template

    tooltipElement?: HTMLElement;

    constructor(private el: ElementRef, private renderer: Renderer2) {}

    // Mouse enters the element
    @HostListener('mouseenter') onMouseEnter() {
        if (!this.tooltipElement) {  // Only create the tooltip if it doesn't exist
            this.showTooltip();
        }
    }

    // Mouse leaves the element
    @HostListener('mouseleave') onMouseLeave() {
        if (this.tooltipElement) {  // Only remove if tooltip exists
            this.hideTooltip();
        }
    }

    showTooltip() {
        this.tooltipElement = this.renderer.createElement('span');
        this.tooltipElement!.innerText = this.tooltipText;

        this.renderer.appendChild(document.body, this.tooltipElement);  // Add tooltip to the body

        // Apply some basic styles
        this.renderer.setStyle(this.tooltipElement, 'position', 'absolute');
        this.renderer.setStyle(this.tooltipElement, 'background-color', '#333');
        this.renderer.setStyle(this.tooltipElement, 'color', '#fff');
        this.renderer.setStyle(this.tooltipElement, 'padding', '5px');
        this.renderer.setStyle(this.tooltipElement, 'z-index', '200');
        this.renderer.setStyle(this.tooltipElement, 'border-radius', '4px');
        this.renderer.setStyle(this.tooltipElement, 'font-size', '12px');
        this.renderer.setStyle(this.tooltipElement, 'top', `${this.el.nativeElement.getBoundingClientRect().top + window.scrollY - this.el.nativeElement.offsetHeight - 10}px`);
        this.renderer.setStyle(this.tooltipElement, 'left', `${this.el.nativeElement.getBoundingClientRect().left + window.scrollX}px`);
    }

    hideTooltip() {
        this.renderer.removeChild(document.body, this.tooltipElement);  // Remove tooltip from the body
        this.tooltipElement = undefined;
    }
}
