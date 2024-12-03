import { ApplicationConfig, provideZoneChangeDetection } from '@angular/core';
import { provideRouter } from '@angular/router';
import { environment } from './../environments/environment';
import {
  HttpRequest,
  HttpHandlerFn,
  HttpEvent,
  withInterceptors,
} from '@angular/common/http';
import { Observable } from 'rxjs';
import { isDevMode, inject } from '@angular/core';
import { AuthService } from './login/auth.service';

import { routes } from './app.routes';
import { provideHttpClient } from '@angular/common/http';
export function apiBaseInterceptor(
  req: HttpRequest<unknown>,
  next: HttpHandlerFn,
): Observable<HttpEvent<unknown>> {
  let baseUrl = environment.baseUrl;
  const apiReq = req.clone({ url: `${baseUrl}${req.url}` });
  return next(apiReq);
}

export function authIntercept(
  req: HttpRequest<unknown>,
  next: HttpHandlerFn,
): Observable<HttpEvent<unknown>> {
  let authSvc: AuthService = inject(AuthService);
  if (authSvc.loggedIn) {
    req = req.clone({
      setHeaders: {
        Authorization: `Bearer ${authSvc.getToken()}`,
      },
    });
  }

  return next(req);
}

export const appConfig: ApplicationConfig = {
  providers: [
    provideZoneChangeDetection({ eventCoalescing: true }),
    provideRouter(routes),
    provideHttpClient(withInterceptors([apiBaseInterceptor, authIntercept])),
  ],
};
