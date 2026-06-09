import { ComponentFixture, TestBed } from '@angular/core/testing';

import { CallsComponent } from './calls.component';

describe('CallsComponent', () => {
  let component: CallsComponent;
  let fixture: ComponentFixture<CallsComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [CallsComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(CallsComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
