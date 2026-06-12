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
  systemName?: string;
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

/**
 * JSON for push.SubscriptionSet (GET/POST body). Matches Go: talkgroups.ID
 * marshals each entry as "system:tgid" string; systems are numeric IDs.
 */
export interface SubscriptionSetJson {
  talkgroups?: string[];
  systems?: number[];
  unsubscribeAll?: boolean | null;
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
    return this.http
      .get<SubscriptionSetJson>('/api/push/subscriptions')
      .pipe(map((body) => this.normalizeSubscriptionSetJson(body)));
  }

  private tgidKey(tg: TGID): string {
    return `${tg.sys}:${tg.tg}`;
  }

  /** Turn wire SubscriptionSetJson into internal SubscriptionSet (TGID[]). */
  private normalizeSubscriptionSetJson(
    body: SubscriptionSetJson | null | undefined,
  ): SubscriptionSet {
    if (!body || typeof body !== 'object') {
      return {
        talkgroups: [],
        systems: [],
        unsubscribeAll: null,
      };
    }
    return {
      talkgroups: this.parseTalkgroupIdList(body.talkgroups),
      systems: this.parseSystemIdList(body.systems),
      unsubscribeAll: body.unsubscribeAll ?? null,
    };
  }

  private parseTalkgroupIdList(raw: unknown): TGID[] {
    if (!raw || !Array.isArray(raw)) {
      return [];
    }
    const out: TGID[] = [];
    for (const item of raw) {
      const id = this.parseOneTalkgroupId(item);
      if (id) {
        out.push(id);
      }
    }
    return out;
  }

  private parseOneTalkgroupId(item: unknown): TGID | null {
    if (typeof item === 'string') {
      const parts = item.split(':');
      if (parts.length < 2) {
        return null;
      }
      const sys = Number(parts[0]);
      const tg = Number(parts[parts.length - 1]);
      if (!Number.isFinite(sys) || !Number.isFinite(tg)) {
        return null;
      }
      return { sys, tg };
    }
    if (item && typeof item === 'object' && 'sys' in item && 'tg' in item) {
      const rec = item as { sys: unknown; tg: unknown };
      const sys = Number(rec.sys);
      const tg = Number(rec.tg);
      if (!Number.isFinite(sys) || !Number.isFinite(tg)) {
        return null;
      }
      return { sys, tg };
    }
    return null;
  }

  private parseSystemIdList(raw: unknown): number[] {
    if (!raw || !Array.isArray(raw)) {
      return [];
    }
    const out: number[] = [];
    for (const item of raw) {
      if (typeof item === 'number' && Number.isFinite(item)) {
        out.push(item);
        continue;
      }
      if (typeof item === 'string') {
        const n = parseInt(item, 10);
        if (Number.isFinite(n)) {
          out.push(n);
        }
      }
    }
    return out;
  }

  subscriptions$(): Observable<TGSubscription[]> {
    // combine the talkgroup list and the subscription map
    const tgs$ = this.tgSvc.allTalkgroups().pipe(catchError(() => of([])));
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
            systemName: tg.system?.name,
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
