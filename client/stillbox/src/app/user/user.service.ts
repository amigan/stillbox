import { HttpClient } from '@angular/common/http';
import { computed, Injectable, Signal, signal } from '@angular/core';
import { AuthService } from '../login/auth.service';
import { Observable } from 'rxjs';
import { toObservable, toSignal } from '@angular/core/rxjs-interop';

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
  public selfUser: Signal<User | undefined>;
  constructor(
    private http: HttpClient,
    private authSvc: AuthService,
  ) {
    this.selfUser = toSignal(this.getSelf());
  }

  getSelf(): Observable<User> {
    return this.http.get<User>('/api/user/');
  }

  changePassword(oldPassword: string, newPassword: string): Observable<void> {
    return this.http.post<void>('/api/user/passwd', {
      oldPassword: oldPassword,
      newPassword: newPassword,
    });
  }
}
