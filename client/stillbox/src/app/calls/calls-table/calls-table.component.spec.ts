import { ComponentFixture, TestBed } from '@angular/core/testing';

import { CallsTableComponent } from './calls-table.component';

describe('CallsTableComponent', () => {
  let component: CallsTableComponent;
  let fixture: ComponentFixture<CallsTableComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [CallsTableComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(CallsTableComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
