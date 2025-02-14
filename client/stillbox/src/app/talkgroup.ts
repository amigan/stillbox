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
  times!: string[];
  mult!: number;

  public getTimes(): AlertTime[] {
    let timesProc = <AlertTime[]>[];
    this.times.forEach((tm) => {
      let sr = tm.split('+');
      timesProc.push(<AlertTime>{
        time: sr[0],
        duration: sr[1],
      });
    });
    return timesProc;
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
  systemId!: number;
  tgid!: number;
  name!: string;
  alphaTag!: string;
  tgGroup!: string;
  frequency!: number;
  metadata!: Metadata | null;
  tags!: string[];
  alert!: boolean;
  system?: System;
  alertRules!: AlertRule[];
  weight!: number;
  learned?: boolean;
  icon?: string;
  iconSvg?: string;
  constructor(
    id: number,
    systemId: number,
    tgid: number,
    name: string,
    alphaTag: string,
    tgGroup: string,
    frequency: number,
    metadata: Metadata | null,
    tags: string[],
    alert: boolean,
    alertRules: AlertRule[],
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
      sys: this.systemId,
      tg: this.tgid,
    };
  }
}

export interface TalkgroupUI extends Talkgroup {
  selected?: boolean;
}

export interface TalkgroupUpdate {
  id: number;
  systemId: number;
  tgid: number;
  name: string | null;
  alphaTag: string | null;
  tgGroup: string | null;
  frequency: number | null;
  metadata: Object | null;
  tags: string[] | null;
  alert: boolean | null;
  alertRules: AlertRule[] | null;
  weight: number | null;
}
