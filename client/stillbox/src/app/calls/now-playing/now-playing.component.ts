import { Component } from '@angular/core';
import { Params, RouterLink } from '@angular/router';
import { PlayerService } from '../player/player.service';
import { TalkgroupPipe } from '../calls.service';
import { AsyncPipe } from '@angular/common';
import { MatIconModule } from '@angular/material/icon';

@Component({
  selector: 'now-playing',
  imports: [TalkgroupPipe, AsyncPipe, MatIconModule, RouterLink],
  templateUrl: './now-playing.component.html',
  styleUrl: './now-playing.component.scss',
})
export class NowPlayingComponent {
  constructor(public playSvc: PlayerService) {}

  /** Query params for the origin link (includes fromQueueOrigin so target can scroll to playing). */
  originLinkQueryParams(): Params {
    const o = this.playSvc.queueOrigin();
    if (!o) return {};
    if (o.type === 'calls') return { ...o.queryParams, fromQueueOrigin: '1' };
    return { fromQueueOrigin: '1' };
  }

  playPause(ev: Event) {
    let playing = this.playSvc.playing() != null;
    let paused = this.playSvc.paused();
    if (playing && !paused) {
      this.playSvc.pauseAudio();
    } else if (playing && paused) {
      this.playSvc.resume();
    }
  }
}
