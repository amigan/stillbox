import { HttpClient } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { SwPush } from '@angular/service-worker';
import { Observable, switchMap } from 'rxjs';

@Injectable({
  providedIn: 'root',
})
export class PushService {
  constructor(
    private http: HttpClient,
    private swPush: SwPush,
  ) {}

  getVapidKey(): Observable<string> {
    return this.http.get<string>('/api/push/vapid');
  }

  postPushSubscriber(sub: PushSubscription): Observable<void> {
    return this.http.post<void>('/api/push/subscribe', sub);
  }

  subscribePush() {
    this.getVapidKey()
      .pipe(
        switchMap((key) =>
          this.swPush
            .requestSubscription({
              serverPublicKey: key,
            })
            .then((sub) => this.postPushSubscriber(sub).subscribe()),
        ),
      )
      .subscribe();
  }
}
