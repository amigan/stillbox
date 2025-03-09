import { Component, computed, input, Input, Signal } from '@angular/core';
import { CallsService, DownloadURLPipe } from '../../calls.service';
import { CallRecord } from '../../../calls';
import { MatIconModule } from '@angular/material/icon';
import { fromEvent, Observable, Subscription } from 'rxjs';
import { PlayerService } from '../player.service';

@Component({
  selector: 'call-player',
  imports: [MatIconModule, DownloadURLPipe],
  templateUrl: './call-player.component.html',
  styleUrl: './call-player.component.scss',
})
export class CallPlayerComponent {
  @Input() call!: CallRecord;
  @Input() index!: number;
  @Input() pause!: boolean|null;
  download = input<boolean>();
  resume = false;

  constructor(private callsSvc: CallsService, private playSvc: PlayerService) {}

  playing: Signal<boolean> = computed(() => this.playSvc.playing()?.id == this.call.id);

  pauseStopAudio(ev: Event) {
    if (this.resume) {
      this.resume = false;
      this.playSvc.resume();
    } else if(this.pause) {
      this.playSvc.pauseAudio();
      this.resume = true;
    } else {
      this.playSvc.stopAudio();
    }
  }

  playAudio(ev: Event) {
    this.resume = false;
    this.playSvc.playAudio(this.call, this.index);
  }
}
