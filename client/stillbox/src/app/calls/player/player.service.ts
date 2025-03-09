import { Injectable, signal } from '@angular/core';
import { fromEvent, Observable, Subscription } from 'rxjs';
import { CallRecord } from '../../calls';
import { CallsService } from '../calls.service';

interface IStack<T> {
  push(e: T[]): void;
  pop(): T | undefined;
  size(): number;
  cancel(): void;
}

class Stack<T> implements IStack<T> {
  private storage: T[] = [];

  constructor() {}

  push(items: T[]): void {
    this.storage.push(...items);
  }
  pop(): T | undefined {
    return this.storage.pop();
  }
  size(): number {
    return this.storage.length;
  }
  cancel(): void {
    this.storage = [];
  }
}

@Injectable({
  providedIn: 'root'
})
export class PlayerService {
  playSub!: Subscription;
  au!: HTMLAudioElement;
  public playing = signal<CallRecord|null>(null);
  stack = new Stack<CallRecord>();
  results = <CallRecord[]>[];

  constructor(private callsSvc: CallsService) { 
    this.au = new Audio();
    this.playSub = fromEvent(this.au, 'ended').subscribe((ev) => this.playNext());
  }

  playNext() {
    if(this.stack.size() > 0) {
      this.play(this.stack.pop()!);
    } else {
      this.playing.set(null);
    }
  }

  stopAudio() {
    this.playing.set(null);
    this.au.pause();
  }

  pauseAudio() {
    this.au.pause();
  }

  playAudio(call: CallRecord, index: number) {
    if(this.playing() != null) {
      this.stopAudio();
    }
    this.stack.cancel();
    this.stack.push(this.results.slice(0, index+1));
    this.playNext();
  }

  resume() {
    this.au.play();
  }

  play(call: CallRecord) {
    this.playing.set(call);
    if (call.audioURL != null) {
      this.au.src = call.audioURL;
    } else {
      this.au.src = this.callsSvc.callAudioURL(call.id);
    }
    this.au.load();
    this.au.play().then(null, (reason) => {
      this.playing.set(null);
    });
  }
}
