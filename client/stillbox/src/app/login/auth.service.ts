import {
  Injectable,
  signal,
  computed,
  effect,
  inject,
  DestroyRef,
} from '@angular/core';
import { HttpClient, HttpResponse } from '@angular/common/http';
import { Router } from '@angular/router';
import { Observable, Subject } from 'rxjs';
import { tap } from 'rxjs/operators';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';

export class Jwt {
  constructor(public jwt: string) {}
}

type AuthState = {
  user: string | null;
  token: string | null;
  is_auth: boolean;
};

@Injectable({
  providedIn: 'root',
})
export class AuthService {
  private _accessTokenKey = 'jwt';
  private _storedToken = localStorage.getItem(this._accessTokenKey);
  destroyed = inject(DestroyRef);

  private _state = signal<AuthState>({
    user: null,
    token: this._storedToken,
    is_auth: this._storedToken !== null,
  });
  loginFailed = signal<boolean>(false);
  token = computed(() => this._state().token);
  isAuth = computed(() => this._state().is_auth);
  user = computed(() => this._state().user);
  constructor(
    private http: HttpClient,
    private _router: Router,
  ) {
    effect(() => {
      const token = this.token();
      if (token !== null) {
        localStorage.setItem(this._accessTokenKey, token);
      } else {
        localStorage.removeItem(this._accessTokenKey);
      }
    });
  }

  login(username: string, password: string) {
    return this.http
      .post<Jwt>('/api/login', { username: username, password: password })
      .pipe(takeUntilDestroyed(this.destroyed))
      .subscribe({
        next: (res) => {
          let state = <AuthState>{
            user: username,
            token: res.jwt,
            is_auth: true,
          };
          this._state.set(state);
          this.loginFailed.update(() => false);
          this._router.navigateByUrl('/');
        },
        error: (err) => {
          this.loginFailed.update(() => true);
        },
      });
  }

  _clearState() {
    this._state.set(<AuthState>{
      is_auth: false,
      token: null,
    });
  }

  logout() {
    this.http.get('/api/logout', { withCredentials: true }).subscribe({
      next: (event) => {
        this._clearState();
      },
      error: (err) => {
        this._clearState();
      },
    });
    this._router.navigateByUrl('/login');
  }

  refresh(): Observable<HttpResponse<Jwt>> {
    return this.http
      .get<Jwt>('/api/refresh', { withCredentials: true, observe: 'response' })
      .pipe(
        tap((event) => {
          if (event.status == 200) {
            let ost = this._state();
            let tok = event.body?.jwt.toString();
            let state = <AuthState>{
              user: ost.user,
              token: tok ? tok : null,
              is_auth: true,
            };
            this._state.set(state);
          }
        }),
      );
  }

  getToken(): string | null {
    return localStorage.getItem(this._accessTokenKey);
  }
}
