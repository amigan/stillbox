import { ComponentFixture, TestBed } from '@angular/core/testing';

import { TalkgroupsComponent } from './talkgroups.component';

describe('TalkgroupsComponent', () => {
  let component: TalkgroupsComponent;
  let fixture: ComponentFixture<TalkgroupsComponent>;

  beforeEach(async () => {
    await TestBed.configureTestingModule({
      imports: [TalkgroupsComponent],
    }).compileComponents();

    fixture = TestBed.createComponent(TalkgroupsComponent);
    component = fixture.componentInstance;
    fixture.detectChanges();
  });

  it('should create', () => {
    expect(component).toBeTruthy();
  });
});
