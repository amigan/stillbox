import { HttpClient } from '@angular/common/http';
import { Injectable, Signal } from '@angular/core';
import { Observable, shareReplay } from 'rxjs';
import { toSignal } from '@angular/core/rxjs-interop';

export interface User {
  uid: number;
  username: string;
  realName: string | null;
  email: string;
  roles: string[] | null;
  lastLoginAt: Date | null;
  lastLoginFrom: string | null;
  isAdmin: boolean | null;
}

@Injectable({
  providedIn: 'root',
})
export class UserService {
  private cache = new Map<string, Observable<User>>();
  private readonly self$: Observable<User>;
  public selfUser: Signal<User | undefined>;
  constructor(private http: HttpClient) {
    this.self$ = this.http
      .get<User>('/api/user/')
      .pipe(shareReplay({ bufferSize: 1, refCount: false }));
    this.selfUser = toSignal(this.self$);
  }

  getSelf(): Observable<User> {
    return this.self$;
  }

  changePassword(oldPassword: string, newPassword: string): Observable<void> {
    return this.http.post<void>('/api/user/passwd', {
      oldPassword: oldPassword,
      newPassword: newPassword,
    });
  }

  setCacheUser(username: string, user: Observable<User>) {
    const existing = this.cache.has(username);
    this.cache.set(username, user);
  }

  getCacheUser(username: string): Observable<User> | undefined {
    const user = this.cache.get(username);
    if (!user) {
      return undefined;
    }

    return user;
  }

  _getUser(username: string): Observable<User> {
    return this.http.get<User>(`/api/user/${username}`);
  }

  getUser(username: string): Observable<User> {
    return this.getCachedUser(username, this._getUser(username));
  }

  getCachedUser(
    username: string,
    fallback: Observable<User>,
  ): Observable<User> {
    const existing = this.getCacheUser(username);
    if (existing) {
      return existing;
    }

    this.setCacheUser(
      username,
      fallback.pipe(shareReplay({ bufferSize: 1, refCount: false })),
    );
    return this.getCacheUser(username)!;
  }
}
