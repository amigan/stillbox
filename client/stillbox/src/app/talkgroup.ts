export interface TGID {
  sys: number;
  tg: number;
}

export class AlertTime {
  time: string;
  duration: string;

  constructor(at: string) {
    let s = at.split('+');
    this.time = s[0];
    this.duration = s[1];
  }
}

export class AlertRule {
  times: AlertTime[];
  mult: number;

  constructor(times: AlertTime[], mult: number) {
    this.times = times;
    this.mult = mult;
  }
}

export interface System {
  id: number;
  name: string;
}

export interface Metadata {
  encrypted: boolean | null;
  icon: string | null;
}

export interface IconMap {
  [name: string]: string;
}

export const iconMapping: IconMap = {
  Ambulance: 'emergency',
  'Dump Truck': '',
  'Fire Truck': 'fire_truck',
  'Garbage Truck': '',
  Police: 'local_police',
  'Transport Bus': 'directions_bus',
  Van: '',
  '': 'group_work',
};

export class Talkgroup {
  id!: number;
  system_id!: number;
  tgid!: number;
  name!: string;
  alpha_tag!: string;
  tg_group!: string;
  frequency!: number;
  metadata!: Metadata | null;
  tags!: string[];
  alert!: boolean;
  system?: System;
  alert_rules!: AlertRule[];
  weight!: number;
  learned?: boolean;
  icon?: string;
  iconSvg?: string;
  constructor(
    id: number,
    system_id: number,
    tgid: number,
    name: string,
    alpha_tag: string,
    tg_group: string,
    frequency: number,
    metadata: Metadata | null,
    tags: string[],
    alert: boolean,
    alert_rules: AlertRule[],
    weight: number,
    system?: System,
    learned?: boolean,
    icon?: string,
  ) {
    this.iconSvg = this.iconMap(this.metadata?.icon!);
  }

  iconMap(icon: string): string {
    return iconMapping[icon]!;
  }

  tgTuple(): TGID {
    return <TGID>{
      sys: this.system_id,
      tg: this.tgid,
    };
  }
}

export interface TalkgroupUI extends Talkgroup {
  selected?: boolean;
}

export interface TalkgroupUpdate {
  id: number;
  system_id: number;
  tgid: number;
  name: string | null;
  alpha_tag: string | null;
  tg_group: string | null;
  frequency: number | null;
  metadata: Object | null;
  tags: string[] | null;
  alert: boolean | null;
  alert_rules: AlertRule[] | null;
  weight: number | null;
}
