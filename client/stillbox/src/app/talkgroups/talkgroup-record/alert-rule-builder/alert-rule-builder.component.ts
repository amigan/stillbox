import {
  Component,
  Input,
  Output,
  EventEmitter,
  Pipe,
  PipeTransform,
} from '@angular/core';
import { AlertRule, AlertTime } from '../../../talkgroup';

@Pipe({
  name: 'ruleProc',
  standalone: true,
  pure: true,
})
export class AlertRulePipe implements PipeTransform {
  transform(rule: AlertRule, args?: any): AlertTime[] {
    let tm = new AlertRule(rule.times, rule.mult);
    return tm.timesProc;
  }
}

@Component({
  selector: 'alert-rule-builder',
  imports: [AlertRulePipe],
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
