import { Component, ElementRef, Input } from '@angular/core';
import * as d3 from 'd3';
import { CallsService } from '../calls/calls.service';
import { Observable, switchMap } from 'rxjs';
import { MatProgressSpinnerModule } from '@angular/material/progress-spinner';

@Component({
  selector: 'chart',
  imports: [MatProgressSpinnerModule],
  templateUrl: './charts.component.html',
  styleUrl: './charts.component.scss',
})
export class ChartsComponent {
  @Input() interval!: Observable<string>;
  loading = true;
  // I hate javascript so much
  months = [
    'Jan',
    'Feb',
    'Mar',
    'Apr',
    'May',
    'Jun',
    'Jul',
    'Aug',
    'Sep',
    'Oct',
    'Nov',
    'Dec',
  ];
  constructor(
    private elementRef: ElementRef,
    private callsSvc: CallsService,
  ) {}

  dateFormat(d: Date, interval: string): string {
    switch (interval) {
      case 'month':
        return `${this.months[d.getMonth()]} ${d.getFullYear()}`;
      case 'day':
        return `${this.months[d.getMonth()]} ${d.getDate()}`;
      case 'week':
        return `${this.months[d.getMonth()]} ${d.getDate()}`;
    }
    return d.toDateString();
  }

  ngOnInit() {
    this.interval
      .pipe(
        switchMap((intv) => {
          d3.select(this.elementRef.nativeElement).select('.chart').html('');
          this.loading = true;
          return this.callsSvc.getCallStats(intv);
        }),
      )
      .subscribe((stats) => {
        let cMax = 0;
        var cMin = 0;
        let data = stats.stats.map((rec) => {
          if (cMin == 0 && rec.count > cMin) {
            cMin = rec.count;
          }
          if (rec.count < cMin) {
            cMin = rec.count;
          }
          if (rec.count > cMax) {
            cMax = rec.count;
          }
          return {
            count: rec.count,
            time: this.dateFormat(new Date(rec.time), stats.interval),
          };
        });
        // set the dimensions and margins of the graph
        var margin = { top: 30, right: 30, bottom: 70, left: 60 },
          widthVal = 600,
          heightVal = 250;
        // clear the old one
        d3.select(this.elementRef.nativeElement).select('.chart').html('');
        let svg = d3
          .select(this.elementRef.nativeElement)
          .select('.chart')
          .append('svg')
          .attr('preserveAspectRatio', 'xMinYMin meet')
          .attr('viewBox', `0 0 ${widthVal} ${heightVal}`)
          .append('g')
          .attr(
            'transform',
            'translate(' + margin.left + ',' + margin.top + ')',
          );

        const width = widthVal - margin.left - margin.right;
        const height = heightVal - margin.top - margin.bottom;
        // X axis
        var x = d3
          .scaleBand()
          .range([0, width])
          .domain(
            data.map(function (d) {
              return d.time;
            }),
          )
          .padding(0.2);
        svg
          .append('g')
          .attr('transform', 'translate(0,' + height + ')')
          .call(d3.axisBottom(x))
          .selectAll('text')
          .attr('transform', 'translate(-10,0)rotate(-45)')
          .style('text-anchor', 'end');

        // Add Y axis
        var y = d3.scaleLinear().domain([0, cMax]).range([height, 0]);
        svg.append('g').call(d3.axisLeft(y));
        svg
          .selectAll('mybar')
          .data(data)
          .enter()
          .append('rect')
          .attr('x', (d) => x(d.time)!)
          .attr('y', function (d) {
            return y(d.count);
          })
          .attr('width', x.bandwidth())
          .attr('height', function (d) {
            return height - y(d.count);
          })
          .attr('fill', function (d) {
            return d3.interpolateTurbo((d.count - cMin) / (cMax - cMin));
          });
        this.loading = false;
      });
  }
}
