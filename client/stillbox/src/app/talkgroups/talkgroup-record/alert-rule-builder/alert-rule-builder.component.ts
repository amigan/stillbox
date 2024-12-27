import { Component, Input, Output, EventEmitter } from '@angular/core';
import { AlertRule } from '../../../talkgroup';

@Component({
  selector: 'alert-rule-builder',
  imports: [],
  templateUrl: './alert-rule-builder.component.html',
  styleUrl: './alert-rule-builder.component.scss',
})
export class AlertRuleBuilderComponent {
  @Input() rules: AlertRule[] = [];
  @Output() rulesChange: EventEmitter<AlertRule[]> = new EventEmitter<
    AlertRule[]
  >();

  emit() {
    this.rulesChange.emit(this.rules);
  }
}
