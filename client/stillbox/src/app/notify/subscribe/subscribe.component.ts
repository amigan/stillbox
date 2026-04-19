import { Component } from '@angular/core';
import { PushService } from '../push.service';

@Component({
  selector: 'push-subscribe',
  imports: [],
  templateUrl: './subscribe.component.html',
  styleUrl: './subscribe.component.scss',
})
export class PushSubscribeComponent {
  constructor(public pushSvc: PushService) {}

  pushSubscribe() {
    this.pushSvc.subscribePush();
  }
}
