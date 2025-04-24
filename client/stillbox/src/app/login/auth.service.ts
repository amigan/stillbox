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
import { Observable, Subject, throwError } from 'rxjs';
import { catchError, tap } from 'rxjs/operators';
import { takeUntilDestroyed } from '@angular/core/rxjs-interop';

export class Jwt {
  constructor(
    public accessToken: string,
    public refreshToken: string,
  ) {}
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
  private _accessTokenKey = 'accessToken';
  private _refreshTokenKey = 'refreshToken';
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
      .subscribe({
        next: (res) => {
          let state = <AuthState>{
            user: username,
            token: res.accessToken,
            is_auth: true,
          };
          localStorage.setItem(this._refreshTokenKey, res.refreshToken);
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
    localStorage.removeItem(this._accessTokenKey);
    localStorage.removeItem(this._refreshTokenKey);
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

  clearAndLoginPage() {
    this._clearState();
    this._router.navigateByUrl('/login');
  }

  refresh(): Observable<Jwt> {
    const refreshToken = localStorage.getItem('refreshToken');
    return this.http
      .get<Jwt>('/api/refresh', {
        headers: {
          Authorization: `Bearer ${refreshToken}`,
        },
      })
      .pipe(
        tap((response) => {
          // Update the access token in the local storage
          localStorage.setItem(this._accessTokenKey, response.accessToken);

          // update the refresh token as well
          localStorage.setItem(this._refreshTokenKey, response.refreshToken);
          let ost = this._state();
          let tok = response.accessToken;
          let state = <AuthState>{
            user: ost.user,
            token: tok ? tok : null,
            is_auth: true,
          };
          this._state.set(state);
        }),
        catchError((error) => {
          console.error('Error refreshing access token:', error);
          return throwError(() => error);
        }),
      );
  }

  getAccessToken(): string | null {
    return localStorage.getItem(this._accessTokenKey);
  }
}
