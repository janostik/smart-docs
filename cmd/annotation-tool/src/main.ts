import {bootstrapApplication} from '@angular/platform-browser';
import {AppAnnotationToolComponent} from './app/app-annotation-tool.component';
import {createCustomElement} from "@angular/elements";
import {inject, Injector, provideExperimentalZonelessChangeDetection} from "@angular/core";
import {provideHttpClient, withNoXsrfProtection} from "@angular/common/http";


bootstrapApplication(AppAnnotationToolComponent, {
  providers: [
    provideExperimentalZonelessChangeDetection(),
    provideHttpClient(withNoXsrfProtection())
  ]
})
  .then(moduleRef => {
    // Get the injector from the module
    const injector = moduleRef.injector;
    const myElement = createCustomElement(AppAnnotationToolComponent, {injector: injector});
    // Define the custom element
    customElements.define('annotation-tool', myElement);
  })
  .catch((err) => console.error(err));
