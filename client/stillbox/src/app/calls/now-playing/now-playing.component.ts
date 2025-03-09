import { Component } from '@angular/core';
import { PlayerService } from '../player/player.service';
import { CallPlayerComponent } from '../player/call-player/call-player.component';
import { TalkgroupPipe } from '../calls.service';
import { AsyncPipe } from '@angular/common';

@Component({
  selector: 'now-playing',
  imports: [
    CallPlayerComponent,
    TalkgroupPipe,
    AsyncPipe,
  ],
  templateUrl: './now-playing.component.html',
  styleUrl: './now-playing.component.scss'
})
export class NowPlayingComponent {

  constructor(public playSvc: PlayerService) {}


}
