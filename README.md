# virtual-student-report-card
Final exam project for university

## Database
For database, I choose `postgresql` mainly because I have previous experience with 
it and know that the integration with `Go` through 
[sqlx](https://github.com/jmoiron/sqlx) and the 
[postgres driver for go](https://github.com/lib/pq) is quite good.

Taking advantage ot the previously mentioned connection I am using the `code-first` 
approach for creating tables and populating them with test information.

![](/Users/Iliyan.Borisov/Downloads/uni-db-1.png)