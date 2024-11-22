import { ComponentFixture, TestBed } from '@angular/core/testing';

import { AlertRuleBuilderComponent } from './alert-rule-builder.component';

describe('AlertRuleBuilderComponent', () => {
  let component: AlertRuleBuilderComponent;
  let fixture: ComponentFixture<AlertRuleBuilderComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [AlertRuleBuilderComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(AlertRuleBuilderComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
