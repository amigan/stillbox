import { Component, effect, signal } from '@angular/core';
import { Params, Router, RouterLink } from '@angular/router';
import { PlayerService } from '../player/player.service';
import { TalkgroupPipe } from '../calls.service';
import { AsyncPipe } from '@angular/common';
import { MatIconModule } from '@angular/material/icon';
import { CallRecord } from '../../calls';

@Component({
  selector: 'now-playing',
  imports: [TalkgroupPipe, AsyncPipe, MatIconModule, RouterLink],
  templateUrl: './now-playing.component.html',
  styleUrl: './now-playing.component.scss',
})
export class NowPlayingComponent {
  public displayCall = signal<CallRecord | null>(null);
  public graceActive = signal<boolean>(false);

  private lastCall: CallRecord | null = null;
  private graceTimeout: ReturnType<typeof setTimeout> | null = null;

  constructor(
    public playSvc: PlayerService,
    private router: Router,
  ) {
    // When playback stops/ends, keep showing the last call for a short
    // "fade out" period before clearing it into idle.
    effect(() => {
      const current = this.playSvc.playing();

      if (current) {
        this.lastCall = current;
        this.displayCall.set(current);
        this.graceActive.set(false);
        if (this.graceTimeout) {
          clearTimeout(this.graceTimeout);
          this.graceTimeout = null;
        }
        return;
      }

      // Idle on startup or if we never had a last call.
      if (!this.lastCall) {
        this.displayCall.set(null);
        this.graceActive.set(false);
        return;
      }

      // Start the "grace" period once after playback goes null.
      if (!this.graceActive()) {
        this.displayCall.set(this.lastCall);
        this.graceActive.set(true);
        this.graceTimeout = setTimeout(() => {
          this.displayCall.set(null);
          this.graceActive.set(false);
          this.lastCall = null;
          this.graceTimeout = null;
        }, 10_000);
      }
    });
  }

  /** Query params for the origin link destination (never includes internal jump flags). */
  originLinkQueryParams(): Params {
    const o = this.playSvc.queueOrigin();
    if (!o) return {};
    if (o.type === 'calls') return { ...o.queryParams };
    return {};
  }

  /** Navigation state marker used by destination screens to scroll to the playing row. */
  originLinkState(): { fromQueueOrigin: true } {
    return { fromQueueOrigin: true };
  }

  /** RouterLink commands for the origin link target. */
  originLinkCommands(): string[] {
    const o = this.playSvc.queueOrigin();
    if (!o) return ['/calls'];
    if (o.type === 'incident') return ['/incidents', o.id];
    if (o.type === 'share') return ['/s', o.id];
    return ['/calls'];
  }

  /**
   * If we're already at the exact origin URL, routerLink won't re-navigate.
   * In that case, perform an immediate in-page scroll to the playing call row.
   */
  jumpToOrigin(ev: Event): void {
    const target = this.router.serializeUrl(
      this.router.createUrlTree(this.originLinkCommands(), {
        queryParams: this.originLinkQueryParams(),
      }),
    );
    const current = this.router.serializeUrl(this.router.parseUrl(this.router.url));
    if (target !== current) return;

    ev.preventDefault();
    ev.stopPropagation();

    const callId = this.playSvc.playing()?.id ?? this.displayCall()?.id;
    if (!callId) return;
    const row = document.querySelector<HTMLElement>(
      `tr[data-call-id="${CSS.escape(callId)}"]`,
    );
    row?.scrollIntoView({ block: 'center', behavior: 'smooth' });
  }

  playPause(ev: Event) {
    ev.preventDefault();
    ev.stopPropagation();
    // Disable controls while idle (including during the grace period).
    if (this.playSvc.playing() == null) return;

    let playing = this.playSvc.playing() != null;
    let paused = this.playSvc.paused();
    if (playing && !paused) {
      this.playSvc.pauseAudio();
    } else if (playing && paused) {
      this.playSvc.resume();
    }
  }

  stop(ev: Event) {
    ev.preventDefault();
    ev.stopPropagation();
    if (this.playSvc.playing() == null) return;
    this.playSvc.stop();
  }
}
