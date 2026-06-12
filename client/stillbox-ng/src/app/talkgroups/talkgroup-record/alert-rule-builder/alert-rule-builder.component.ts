import {
  Component,
  Input,
  Output,
  EventEmitter,
  Pipe,
  PipeTransform,
} from '@angular/core';
import { AlertRule, AlertTime } from '../../../talkgroup';
import { MatTableModule } from '@angular/material/table';

@Pipe({
  name: 'ruleTimes',
  standalone: true,
  pure: true,
})
export class AlertRulePipe implements PipeTransform {
  transform(rule: AlertRule, args?: any): AlertTime[] {
    return rule.getTimes();
  }
}

@Component({
  selector: 'alert-rule-builder',
  imports: [MatTableModule, AlertRulePipe],
  templateUrl: './alert-rule-builder.component.html',
  styleUrl: './alert-rule-builder.component.scss',
})
export class AlertRuleBuilderComponent {
  @Input() rules!: AlertRule[];
  @Output() rulesChange: EventEmitter<AlertRule[]> = new EventEmitter<
    AlertRule[]
  >();

  displayedColumns = ['time', 'duration', 'multiplier'];

  ngOnInit() {
    this.rules = <AlertRule[]>this.rules;
  }

  emit() {
    this.rulesChange.emit(this.rules);
  }
}
