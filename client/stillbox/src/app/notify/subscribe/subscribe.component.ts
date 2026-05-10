import { Component } from '@angular/core';
import { PushService, TGSubscription } from '../push.service';
import { MatIcon } from '@angular/material/icon';
import { BehaviorSubject, catchError, finalize, map, Observable, of, switchMap } from 'rxjs';
import { CommonModule } from '@angular/common';
import { TGID } from '../../talkgroup';

interface TalkgroupSubscriptionView extends TGSubscription {
  key: string;
}

interface SystemSubscriptionView {
  systemId: number;
  talkgroups: TalkgroupSubscriptionView[];
  total: number;
  subscribedCount: number;
  allSubscribed: boolean;
  someSubscribed: boolean;
}

@Component({
  selector: 'push-subscribe',
  imports: [MatIcon, CommonModule],
  templateUrl: './subscribe.component.html',
  styleUrl: './subscribe.component.scss',
})
export class PushSubscribeComponent {
  private readonly refresh$ = new BehaviorSubject<void>(undefined);
  private readonly filter$ = new BehaviorSubject<string>('');
  private readonly talkgroupStateOverrides = new Map<
    string,
    { subscribed: boolean; viaSystemSub: boolean }
  >();

  loading = false;
  errorMessage = '';
  vm$!: Observable<SystemSubscriptionView[]>;

  constructor(public pushSvc: PushService) {}

  ngOnInit() {
    this.vm$ = this.refresh$.pipe(
      switchMap(() =>
        this.pushSvc.subscriptions$().pipe(
          catchError(() => {
            this.errorMessage = 'Unable to load subscriptions.';
            return of([]);
          }),
        ),
      ),
      switchMap((subs) =>
        this.filter$.pipe(
          map((filter) => this.buildViewModel(subs, filter)),
        ),
      ),
    );
  }

  pushSubscribe() {
    this.pushSvc.subscribePush();
  }

  setFilter(ev: Event) {
    const value = (ev.target as HTMLInputElement).value ?? '';
    this.filter$.next(value.trim().toLowerCase());
  }

  toggleSystem(system: SystemSubscriptionView) {
    this.errorMessage = '';
    this.loading = true;
    const op$ = system.allSubscribed
      ? this.pushSvc.unsubscribeSystem(system.systemId)
      : this.pushSvc.subscribeSystem(system.systemId);
    op$
      .pipe(finalize(() => (this.loading = false)))
      .subscribe({
        next: () => {
          this.clearOverridesForSystem(system.systemId);
          this.refresh();
        },
        error: () => (this.errorMessage = 'Failed to update system subscription.'),
      });
  }

  toggleTalkgroup(tg: TalkgroupSubscriptionView) {
    if (tg.viaSystemSub) {
      return;
    }
    this.errorMessage = '';
    this.loading = true;
    const op$ = tg.subscribed
      ? this.pushSvc.unsubscribeTalkgroup(tg.tg)
      : this.pushSvc.subscribeTalkgroup(tg.tg);
    op$
      .pipe(finalize(() => (this.loading = false)))
      .subscribe({
        next: () => {
          this.talkgroupStateOverrides.set(tg.key, {
            subscribed: !tg.subscribed,
            viaSystemSub: false,
          });
          this.refresh();
        },
        error: () => (this.errorMessage = 'Failed to update talkgroup subscription.'),
      });
  }

  clearAll() {
    this.errorMessage = '';
    this.loading = true;
    this.pushSvc
      .unsubscribeSubscriptions()
      .pipe(finalize(() => (this.loading = false)))
      .subscribe({
        next: () => {
          this.talkgroupStateOverrides.clear();
          this.refresh();
        },
        error: () => (this.errorMessage = 'Failed to clear subscriptions.'),
      });
  }

  trackSystem(_: number, system: SystemSubscriptionView): number {
    return system.systemId;
  }

  trackTalkgroup(_: number, tg: TalkgroupSubscriptionView): string {
    return tg.key;
  }

  tooltipForTalkgroup(tg: TalkgroupSubscriptionView): string {
    const group = (tg.tgGroup ?? '').trim() || 'Unknown group';
    const name = (tg.name ?? '').trim() || 'Unknown talkgroup';
    const id = `${tg.tg.sys}:${tg.tg.tg}`;
    return `${group} - ${name} (${id})`;
  }

  talkgroupLabel(tg: TalkgroupSubscriptionView): string {
    const alphaTag = (tg.alphaTag ?? '').trim();
    if (alphaTag) {
      return alphaTag;
    }
    const name = (tg.name ?? '').trim();
    if (name) {
      return name;
    }
    return `${tg.tg.sys}:${tg.tg.tg}`;
  }

  private refresh() {
    this.refresh$.next();
  }

  private clearOverridesForSystem(systemId: number) {
    for (const key of this.talkgroupStateOverrides.keys()) {
      if (key.startsWith(`${systemId}:`)) {
        this.talkgroupStateOverrides.delete(key);
      }
    }
  }

  private tgKey(tg: TGID): string {
    return `${tg.sys}:${tg.tg}`;
  }

  private buildViewModel(
    subs: TGSubscription[],
    filter: string,
  ): SystemSubscriptionView[] {
    const grouped = new Map<number, TalkgroupSubscriptionView[]>();

    for (const sub of subs) {
      const alphaTag = (sub.alphaTag ?? '').trim();
      const name = (sub.name ?? '').trim();
      const tgGroup = (sub.tgGroup ?? '').trim();
      const tags = (sub.tags ?? []).join(' ').trim();
      const haystack = `${alphaTag} ${name} ${tgGroup} ${tags}`.toLowerCase();
      if (filter && !haystack.includes(filter)) {
        continue;
      }
      const tgRow: TalkgroupSubscriptionView = {
        ...sub,
        alphaTag,
        name,
        tgGroup,
        tags: sub.tags ?? [],
        key: this.tgKey(sub.tg),
      };
      const override = this.talkgroupStateOverrides.get(tgRow.key);
      if (override) {
        tgRow.subscribed = override.subscribed;
        tgRow.viaSystemSub = override.viaSystemSub;
      }
      if (!grouped.has(sub.tg.sys)) {
        grouped.set(sub.tg.sys, []);
      }
      grouped.get(sub.tg.sys)!.push(tgRow);
    }

    return [...grouped.entries()]
      .map(([systemId, talkgroups]) => {
        const sortedTalkgroups = [...talkgroups].sort((a, b) =>
          (a.alphaTag ?? '').localeCompare(b.alphaTag ?? ''),
        );
        const subscribedCount = sortedTalkgroups.filter((tg) => tg.subscribed).length;
        const total = sortedTalkgroups.length;
        return <SystemSubscriptionView>{
          systemId,
          talkgroups: sortedTalkgroups,
          total,
          subscribedCount,
          allSubscribed: total > 0 && subscribedCount === total,
          someSubscribed: subscribedCount > 0 && subscribedCount < total,
        };
      })
      .sort((a, b) => a.systemId - b.systemId);
  }
}
