import { Component, computed, input, Input, Signal } from '@angular/core';
import { CallsService, DownloadURLPipe } from '../../calls.service';
import { CallRecord } from '../../../calls';
import { MatIconModule } from '@angular/material/icon';
import { fromEvent, Observable, Subscription } from 'rxjs';
import { PlayerService } from '../player.service';
import { Share } from '../../../shares';

@Component({
  selector: 'call-player',
  imports: [MatIconModule, DownloadURLPipe],
  templateUrl: './call-player.component.html',
  styleUrl: './call-player.component.scss',
})
export class CallPlayerComponent {
  @Input() forward: boolean = false;
  @Input() call!: CallRecord;
  @Input() index!: number;
  @Input() share: Share | undefined;
  download = input<boolean>();

  constructor(public playSvc: PlayerService) {}

  playing: Signal<boolean> = computed(
    () => this.playSvc.playing()?.id == this.call.id,
  );

  stopAudio(ev: Event) {
    if (this.playSvc.playing()) {
      this.playSvc.stop();
    }
  }

  playAudio(ev: Event) {
    this.playSvc.playAudio(this.call, this.share, this.index, this.forward);
  }
}
