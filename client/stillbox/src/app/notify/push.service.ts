import { HttpClient } from '@angular/common/http';
import { Injectable } from '@angular/core';
import { SwPush } from '@angular/service-worker';
import { catchError, combineLatest, map, Observable, of, switchMap } from 'rxjs';
import { TalkgroupService } from '../talkgroups/talkgroups.service';
import { System, TGID } from '../talkgroup';

export interface TGSubscription {
  tg: TGID;
  alphaTag: string;
  name?: string;
  tgGroup?: string;
  tags?: string[];
  subscribed: boolean;
  viaSystemSub: boolean;
}

export interface SystemSubscription {
  sys: System;
  subscribed: boolean;
}

export interface SubscriptionSet {
  talkgroups: TGID[];
  systems: number[];

  // unsubscribeAll is only valid for unsubscribing. It is a no-op otherwise.
  // If systems is nil, it unsubscribes everything.
  // If systems is not null, it unsubscribes those systems *AND* and talkgroups under them.
  unsubscribeAll: boolean | null | undefined;
}

@Injectable({
  providedIn: 'root',
})
export class PushService {
  constructor(
    private http: HttpClient,
    private swPush: SwPush,
    private tgSvc: TalkgroupService,
  ) {}

  getVapidKey(): Observable<string> {
    return this.http.get<string>('/api/push/vapid');
  }

  postPushSubscriber(sub: PushSubscription): Observable<void> {
    return this.http.post<void>('/api/push/subscribe', sub);
  }

  subscribeSubscriptions(subSet: SubscriptionSet): Observable<void> {
    return this.http.post<void>('/api/push/subscriptions', subSet);
  }

  unsubscribeSubscriptions(subSet?: SubscriptionSet): Observable<void> {
    return this.http.request<void>('delete', '/api/push/subscriptions', {
      body: subSet,
    });
  }

  subscribeSystem(system: number): Observable<void> {
    return this.http.put<void>(`/api/push/subscriptions/${system}`, null);
  }

  unsubscribeSystem(system: number): Observable<void> {
    return this.unsubscribeSubscriptions({
      talkgroups: [],
      systems: [system],
      unsubscribeAll: true,
    });
  }

  subscribeTalkgroup(tg: TGID): Observable<void> {
    return this.http.put<void>(
      `/api/push/subscriptions/${tg.sys}/${tg.tg}`,
      null,
    );
  }

  unsubscribeTalkgroup(tg: TGID): Observable<void> {
    return this.http.delete<void>(`/api/push/subscriptions/${tg.sys}/${tg.tg}`);
  }

  subscribePush() {
    this.getVapidKey()
      .pipe(
        switchMap((key) => {
          return this.swPush
            .requestSubscription({
              serverPublicKey: key,
            })
            .then((sub) => this.postPushSubscriber(sub).subscribe());
        }),
      )
      .subscribe();
  }

  getSubscriptions(): Observable<SubscriptionSet> {
    return this.http.get<SubscriptionSet>('/api/push/subscriptions');
  }

  private tgidKey(tg: TGID): string {
    return `${tg.sys}:${tg.tg}`;
  }

  subscriptions$(): Observable<TGSubscription[]> {
    // combine the talkgroup list and the subscription map
    const tgs$ = this.tgSvc
      .getTalkgroupsPag({
        page: 1,
        perPage: 10000,
        filter: null,
      })
      .pipe(
        map((res) => res.talkgroups),
        catchError(() => of([])),
      );
    const subs$ = this.getSubscriptions().pipe(
      catchError(() =>
        of({
          talkgroups: [],
          systems: [],
          unsubscribeAll: null,
        } as SubscriptionSet),
      ),
    );
    return combineLatest([tgs$, subs$]).pipe(
      map(([tgs, subs]) => {
        const tgSubs = new Set<string>(
          subs.talkgroups
            ? subs.talkgroups.map((tg) => this.tgidKey(tg))
            : [],
        );
        const sysSubs = new Map<number, boolean>(
          subs.systems ? subs.systems.map((sys) => [sys, true]) : [],
        );
        const res = tgs.map((tg) => {
          const tgid = <TGID>{
            tg: tg.tgid,
            sys: tg.systemId,
          };
          const sysSub = sysSubs.has(tgid.sys);
          return <TGSubscription>{
            tg: tgid,
            alphaTag: tg.alphaTag,
            name: tg.name,
            tgGroup: tg.tgGroup,
            tags: tg.tags,
            subscribed: tgSubs.has(this.tgidKey(tgid)) || sysSub,
            viaSystemSub: sysSub,
          };
        });
        return res;
      }),
    );
  }
}
