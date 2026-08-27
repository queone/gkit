## days
Command line calendar days calculator.

### Why?
Almost all of these days calculations can of course be done with most Unix `date` commands, or with many other tools. So why a dedicated `days` utility? Because these simple calculations seem to occur very often, and having a simple dedicated utility for them is quite handy. Using the `date` command, and/or most other ways, always seems too elaborate. But using this utility, one can quickly, for instance, get someone's age by simply doing the following: 

```bash
days 1995-07-14
-9921 (27 years + 66 days)
```

Or maybe there's a need to quickly calculate how many days and years passed between 2 historical dates or years: 

```bash
days 1492-07-01 1776-12-01
103882 (284 years + 222 days)
```

Or maybe one simply needs what the date was X days ago, or what it will be X days into the future: 

```bash
days -342
2021-10-04

days +90
2022-12-10
```


### Known Issues
- All calculations are based on UTC timezone.

### Getting Started
This utility is part of a collection of Go utilities. To compile and install follow the **Getting Started** instructions at the [gkit repo](https://github.com/queone/gkit).

### Usage

```bash
$ days
days v1.1.0
Calendar days calculator — https://github.com/queone/gkit/blob/main/cmd/days/README.md
Overview
  This utility works with calendar dates expressed as YYYY-MM-DD (or the equivalent
  YYYY-MMM-DD format), and reports the relationship between today's date and the supplied
  argument(s). Supported invocations are:

    days -v, --version            Prints days v1.1.0 and exits.
    days -N                       Prints the calendar date N days ago (e.g. -11).
    days +N                       Prints the calendar date N days in the future (e.g. +6 or just 6).
    days YYYY-MM-DD               Prints the number of days between today and the given date (positive
                                  if the date is in the future, negative if it is in the past).
    days YYYY-MM-DD YYYY-MM-DD    Prints the number of days between the two supplied dates.
```
