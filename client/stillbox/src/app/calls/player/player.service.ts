import { Injectable, signal } from '@angular/core';
import { fromEvent, Observable, Subscription } from 'rxjs';
import { CallRecord } from '../../calls';
import { CallsService } from '../calls.service';

@Injectable({
  providedIn: 'root'
})
export class PlayerService {
  playSub!: Subscription;
  au!: HTMLAudioElement;
  public playing = signal<string|null>(null);
  queue = <CallRecord[]>[];

  constructor(private callsSvc: CallsService) { 
    this.au = new Audio();
    this.playSub = fromEvent(this.au, 'ended').subscribe((ev) => {
      this.playing.set(null);
    });
  }

  stopAudio() {
    this.playing.set(null);
    this.au.pause();
  }

  playAudio(call: CallRecord) {
    if(this.playing() != null) {
      this.stopAudio();
    }

    this.playing.set(call.id);
    if (call.audioURL != null) {
      this.au.src = call.audioURL;
    } else {
      this.au.src = this.callsSvc.callAudioURL(call.id);
    }
    this.au.load();
    this.au.play().then(null, (reason) => {
      this.playing.set(null);
      alert(reason);
    });
  }
}
