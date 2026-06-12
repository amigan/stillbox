import { Component } from '@angular/core';
import { PushSubscribeComponent } from "../notify/subscribe/subscribe.component";

@Component({
  selector: 'app-alerts',
  imports: [PushSubscribeComponent],
  templateUrl: './alerts.component.html',
  styleUrl: './alerts.component.scss',
})
export class AlertsComponent {}
