import { Injectable } from '@angular/core';
import { HttpClient, HttpResponse } from '@angular/common/http';
import { Router } from '@angular/router';
import { Observable } from 'rxjs';
import { tap } from 'rxjs/operators';

export class Jwt {
  constructor(public jwt: string) {}
}

@Injectable({
  providedIn: 'root',
})
export class AuthService {
  loggedIn: boolean = false;
  constructor(
    private http: HttpClient,
    private _router: Router,
  ) {
    let ssJWT = localStorage.getItem('jwt');
    if (ssJWT) {
      this.loggedIn = true;
    }
  }

  login(username: string, password: string): Observable<HttpResponse<Jwt>> {
    return this.http
      .post<Jwt>(
        '/api/login',
        { username: username, password: password },
        { observe: 'response' },
      )
      .pipe(
        tap((event) => {
          if (event.status == 200) {
            localStorage.setItem('jwt', event.body?.jwt.toString() ?? '');
            this.loggedIn = true;
            this._router.navigateByUrl('/home');
          }
        }),
      );
  }

  logout() {
    this.http
      .get('/api/logout', { withCredentials: true, observe: 'response' })
      .subscribe((event) => {
        if (event.status == 200) {
          this.loggedIn = false;
        }
      });
    localStorage.removeItem('jwt');
    this.loggedIn = false;
    this._router.navigateByUrl('/login');
  }

  refresh(): Observable<HttpResponse<Jwt>> {
    return this.http
      .get<Jwt>('/api/refresh', { withCredentials: true, observe: 'response' })
      .pipe(
        tap((event) => {
          if (event.status == 200) {
            localStorage.setItem('jwt', event.body?.jwt.toString() ?? '');
            this.loggedIn = true;
          }
        }),
      );
  }

  getToken(): string | null {
    return localStorage.getItem('jwt');
  }
}
