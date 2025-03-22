import { Component } from '@angular/core';
import { PlayerService } from '../player/player.service';
import { TalkgroupPipe } from '../calls.service';
import { AsyncPipe } from '@angular/common';
import { MatIconModule } from '@angular/material/icon';

@Component({
  selector: 'now-playing',
  imports: [TalkgroupPipe, AsyncPipe, MatIconModule],
  templateUrl: './now-playing.component.html',
  styleUrl: './now-playing.component.scss',
})
export class NowPlayingComponent {
  constructor(public playSvc: PlayerService) {}

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
