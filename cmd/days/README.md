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

```text
days v1.2.0
Count calendar days between dates, or find the date N days away
github.com/queone/gkit/tree/main/cmd/days

Usage
  days -N         Print the date N days ago
  days +N         Print the date N days ahead; a bare N means +N
  days DATE       Print the days from today to DATE, negative when DATE is past
  days DATE DATE  Print the days between the two dates

  DATE is YYYY-MM-DD or YYYY-MMM-DD.

Options
  -v, --version   Print days v1.2.0 and exit
  -h, -?, --help  Show this help and exit

Examples
  days -11         The date eleven days ago
  days 6           The date six days ahead
  days 2026-12-25  Days until that date
```
