import { Component, effect, signal } from '@angular/core';
import { Params, RouterLink } from '@angular/router';
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

  constructor(public playSvc: PlayerService) {
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

  /** Query params for the origin link (includes fromQueueOrigin so target can scroll to playing). */
  originLinkQueryParams(): Params {
    const o = this.playSvc.queueOrigin();
    if (!o) return {};
    if (o.type === 'calls') return { ...o.queryParams, fromQueueOrigin: '1' };
    return { fromQueueOrigin: '1' };
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
