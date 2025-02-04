import { ComponentFixture, TestBed } from '@angular/core/testing';

import { CallInfoComponent } from './call-info.component';

describe('CallInfoComponent', () => {
  let component: CallInfoComponent;
  let fixture: ComponentFixture<CallInfoComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [CallInfoComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(CallInfoComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
