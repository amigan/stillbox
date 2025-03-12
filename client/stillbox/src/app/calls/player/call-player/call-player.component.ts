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
  download = input<boolean>();

  constructor(private callsSvc: CallsService, private playSvc: PlayerService) {}

  playing: Signal<boolean> = computed(() => this.playSvc.playing() == this.call.id);

  stopAudio(ev: Event) {
    this.playSvc.stopAudio();
  }

  playAudio(ev: Event) {
    this.playSvc.playAudio(this.call);
  }
}
