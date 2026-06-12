import { ComponentFixture, TestBed } from '@angular/core/testing';

import { CallPlayerComponent } from './call-player.component';

describe('CallPlayerComponent', () => {
  let component: CallPlayerComponent;
  let fixture: ComponentFixture<CallPlayerComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [CallPlayerComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(CallPlayerComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
