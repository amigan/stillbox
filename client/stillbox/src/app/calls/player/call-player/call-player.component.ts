import { Component, Input } from '@angular/core';
import { CallsService } from '../../calls.service';
import { CallRecord } from '../../../calls';
import { MatIconModule } from '@angular/material/icon';
import { fromEvent, Observable, Subscription } from 'rxjs';

@Component({
  selector: 'call-player',
  imports: [MatIconModule],
  templateUrl: './call-player.component.html',
  styleUrl: './call-player.component.scss',
})
export class CallPlayerComponent {
  @Input() call!: CallRecord;
  playing = false;
  playSub!: Subscription;
  au!: HTMLAudioElement;

  constructor(private callsSvc: CallsService) {}

  stopAudio(ev: Event) {
    this.au.pause();
    this.playing = false;
  }

  playAudio(ev: Event) {
    this.au = new Audio();
    this.playSub = fromEvent(this.au, 'ended').subscribe((ev) => {
      this.playing = false;
      this.playSub.unsubscribe();
    });
    this.playing = true;
    this.au.src = this.callsSvc.callAudioURL(this.call.id);
    this.au.load();
    this.au.play();
  }
}
