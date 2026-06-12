import {
  ApplicationConfig,
  Injectable,
  provideZoneChangeDetection,
} from '@angular/core';
import { provideRouter } from '@angular/router';
import { environment } from './../environments/environment';
import {
  HttpRequest,
  HttpEvent,
  withInterceptors,
  HttpHandlerFn,
} from '@angular/common/http';
import { catchError, Observable, switchMap, throwError } from 'rxjs';
import { isDevMode, inject } from '@angular/core';
import { AuthService } from './login/auth.service';

import { routes } from './app.routes';
import { provideHttpClient } from '@angular/common/http';
import { provideAnimationsAsync } from '@angular/platform-browser/animations/async';
import { provideServiceWorker } from '@angular/service-worker';
import { provideMarkdown } from 'ngx-markdown';
export function apiBaseInterceptor(
  req: HttpRequest<unknown>,
  next: HttpHandlerFn,
): Observable<HttpEvent<unknown>> {
  let baseUrl = environment.baseUrl;
  const apiReq = req.clone({ url: `${baseUrl}${req.url}` });
  return next(apiReq);
}

export function tokenIntercept(
  req: HttpRequest<unknown>,
  next: HttpHandlerFn,
): Observable<HttpEvent<unknown>> {
  if (req.url.indexOf('api/refresh') > 0) {
    return next(req);
  }
  let authSvc: AuthService = inject(AuthService);
  const accessToken = authSvc.getAccessToken();
  if (authSvc.isAuth() && accessToken) {
    req = addToken(req, accessToken);
  }

  return next(req).pipe(
    catchError((error) => {
      if (error.status == 401 && accessToken) {
        return handleExpiredToken(authSvc, req, next);
      }

      return throwError(() => error);
    }),
  );
}

function handleExpiredToken(
  authSvc: AuthService,
  request: HttpRequest<any>,
  next: HttpHandlerFn,
): Observable<HttpEvent<any>> {
  return authSvc.refresh().pipe(
    switchMap(() => {
      const newAccessToken = authSvc.getAccessToken();
      // Retry the original request with the new access token
      return next(addToken(request, newAccessToken!));
    }),
    catchError((error) => {
      // Handle refresh token error (e.g., redirect to login page)
      authSvc.clearAndLoginPage();
      return throwError(() => error);
    }),
  );
}

function addToken(request: HttpRequest<any>, token: string): HttpRequest<any> {
  return request.clone({
    setHeaders: {
      Authorization: `Bearer ${token}`,
    },
  });
}

export const appConfig: ApplicationConfig = {
  providers: [
    provideZoneChangeDetection({ eventCoalescing: true }),
    provideRouter(routes),
    provideHttpClient(withInterceptors([apiBaseInterceptor, tokenIntercept])),
    provideAnimationsAsync(),
    provideServiceWorker('ngsw-worker.js', {
      enabled: !isDevMode(),
      registrationStrategy: 'registerWhenStable:30000',
    }),
    provideMarkdown(),
  ],
};
