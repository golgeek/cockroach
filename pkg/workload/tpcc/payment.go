// Copyright 2017 The Cockroach Authors.
//
// Use of this software is governed by the CockroachDB Software License
// included in the /LICENSE file.

package tpcc

import (
	"context"
	"fmt"
	"math/rand/v2"
	"time"

	"github.com/cockroachdb/cockroach/pkg/util/bufalloc"
	"github.com/cockroachdb/cockroach/pkg/util/log"
	"github.com/cockroachdb/cockroach/pkg/util/timeutil"
	"github.com/cockroachdb/cockroach/pkg/workload"
	"github.com/cockroachdb/errors"
	"github.com/jackc/pgx/v5"
)

// Section 2.5:
//
// The Payment business transaction updates the customer's balance and reflects
// the payment on the district and warehouse sales statistics. It represents a
// light-weight, read-write transaction with a high frequency of execution and
// stringent response time requirements to satisfy on-line users. In addition,
// this transaction includes non-primary key access to the CUSTOMER table.

type paymentData struct {
	// This data must all be returned by the transaction. See 2.5.3.4.
	dID  int
	cID  int
	cDID int
	cWID int

	wStreet1 string
	wStreet2 string
	wCity    string
	wState   string
	wZip     string

	dStreet1 string
	dStreet2 string
	dCity    string
	dState   string
	dZip     string

	cFirst     string
	cMiddle    string
	cLast      string
	cStreet1   string
	cStreet2   string
	cCity      string
	cState     string
	cZip       string
	cPhone     string
	cSince     time.Time
	cCredit    string
	cCreditLim float64
	cDiscount  float64
	cBalance   float64
	cData      string

	hAmount float64
	hDate   time.Time
}

type payment struct {
	config *tpcc
	mcp    *workload.MultiConnPool
	sr     workload.SQLRunner

	// Optimized implementation statements
	updateWarehouse   workload.StmtHandle
	updateDistrict    workload.StmtHandle
	selectByLastName  workload.StmtHandle
	updateWithPayment workload.StmtHandle
	insertHistory     workload.StmtHandle
	resetWarehouse    workload.StmtHandle
	resetDistrict     workload.StmtHandle

	// Literal implementation statements (without RETURNING clause)
	literalUpdateWarehouse          workload.StmtHandle
	literalUpdateDistrict           workload.StmtHandle
	literalSelectWarehouse          workload.StmtHandle
	literalSelectDistrict           workload.StmtHandle
	literalSelectByLastName         workload.StmtHandle
	literalUpdateCustomer           workload.StmtHandle
	literalSelectDataForCustomerBad workload.StmtHandle
	literalUpdateCustomerBad        workload.StmtHandle

	a bufalloc.ByteAllocator
}

var _ tpccTx = &payment{}

func createPayment(ctx context.Context, config *tpcc, mcp *workload.MultiConnPool) (tpccTx, error) {
	p := &payment{
		config: config,
		mcp:    mcp,
	}

	// Update warehouse with payment (optimized with RETURNING)
	p.updateWarehouse = p.sr.Define(`
		UPDATE warehouse
		SET w_ytd = w_ytd + $1
		WHERE w_id = $2
		RETURNING w_name, w_street_1, w_street_2, w_city, w_state, w_zip`,
	)

	// Update district with payment (optimized with RETURNING)
	p.updateDistrict = p.sr.Define(`
		UPDATE district
		SET d_ytd = d_ytd + $1
		WHERE d_w_id = $2 AND d_id = $3
		RETURNING d_name, d_street_1, d_street_2, d_city, d_state, d_zip`,
	)

	// 2.5.2.2 Case 2: Pick the middle row, rounded up, from the selection by last name.
	p.selectByLastName = p.sr.Define(`
		SELECT c_id
		FROM customer
		WHERE c_w_id = $1 AND c_d_id = $2 AND c_last = $3
		ORDER BY c_first ASC`,
	)

	// Update customer with payment (optimized with RETURNING).
	// If the customer has bad credit, update the customer's C_DATA and return
	// the first 200 characters of it, which is supposed to get displayed by
	// the terminal. See 2.5.3.3 and 2.5.2.2.
	p.updateWithPayment = p.sr.Define(`
		UPDATE customer
		SET (c_balance, c_ytd_payment, c_payment_cnt, c_data) =
		(c_balance - ($1:::float)::decimal, c_ytd_payment + ($1:::float)::decimal, c_payment_cnt + 1,
			 case c_credit when 'BC' then
			 left(c_id::text || c_d_id::text || c_w_id::text || ($5:::int)::text || ($6:::int)::text || ($1:::float)::text || c_data, 500)
			 else c_data end)
		WHERE c_w_id = $2 AND c_d_id = $3 AND c_id = $4
		RETURNING c_first, c_middle, c_last, c_street_1, c_street_2,
					c_city, c_state, c_zip, c_phone, c_since, c_credit,
					c_credit_lim, c_discount, c_balance, case c_credit when 'BC' then left(c_data, 200) else '' end`,
	)

	// Insert history line.
	p.insertHistory = p.sr.Define(`
		INSERT INTO history (h_c_id, h_c_d_id, h_c_w_id, h_d_id, h_w_id, h_amount, h_date, h_data)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8)`,
	)

	// Literal implementation statements (without RETURNING clause)

	// Update warehouse with payment (literal without RETURNING)
	p.literalUpdateWarehouse = p.sr.Define(`
		UPDATE warehouse
		SET w_ytd = w_ytd + $1
		WHERE w_id = $2`,
	)

	// Select warehouse data after update
	p.literalSelectWarehouse = p.sr.Define(`
		SELECT w_name, w_street_1, w_street_2, w_city, w_state, w_zip
		FROM warehouse
		WHERE w_id = $1
        FOR UPDATE`,
	)

	// Update district with payment (literal without RETURNING)
	p.literalUpdateDistrict = p.sr.Define(`
		UPDATE district
		SET d_ytd = d_ytd + $1
		WHERE d_w_id = $2 AND d_id = $3`,
	)

	// Select district data after update
	p.literalSelectDistrict = p.sr.Define(`
		SELECT d_name, d_street_1, d_street_2, d_city, d_state, d_zip
		FROM district
		WHERE d_w_id = $1 AND d_id = $2
		FOR UPDATE`,
	)

	// 2.5.2.2 Case 2: Pick the middle row, rounded up, from the selection by last name.
	p.literalSelectByLastName = p.sr.Define(`
		SELECT c_first, c_middle, c_last, c_id, c_street_1, c_street_2,
			c_city, c_state, c_zip, c_phone, c_since, c_credit,
			c_credit_lim, c_discount, c_balance
		FROM customer
		WHERE c_w_id = $1 AND c_d_id = $2 AND c_last = $3
		ORDER BY c_first ASC`,
	)

	// Update customer with payment (for good credit customers)
	p.literalUpdateCustomer = p.sr.Define(`
		UPDATE customer
		SET c_balance = c_balance - ($1:::float)::decimal,
			c_ytd_payment = c_ytd_payment + ($1:::float)::decimal,
			c_payment_cnt = c_payment_cnt + 1
		WHERE c_w_id = $2 AND c_d_id = $3 AND c_id = $4`,
	)

	// Select the CDATA for a customer (for bad credit customers)
	p.literalSelectDataForCustomerBad = p.sr.Define(`
		SELECT c_data
		FROM customer
		WHERE c_w_id = $1 AND c_d_id = $2 AND c_id = $3`,
	)

	// Update customer with payment (for bad credit customers)
	p.literalUpdateCustomerBad = p.sr.Define(`
		UPDATE customer
		SET c_balance = c_balance - ($1:::float)::decimal,
			c_ytd_payment = c_ytd_payment + ($1:::float)::decimal,
			c_payment_cnt = c_payment_cnt + 1,
			c_data = left(c_id::text || c_d_id::text || c_w_id::text || ($5:::int)::text || ($6:::int)::text || ($1:::float)::text || c_data, 500)
		WHERE c_w_id = $2 AND c_d_id = $3 AND c_id = $4`,
	)

	if p.config.isLongDurationWorkload {
		p.resetWarehouse = p.sr.Define(`
		UPDATE warehouse
		SET w_ytd = $1
		WHERE w_id >= $2 AND w_id < $3`,
		)

		p.resetDistrict = p.sr.Define(`
		UPDATE district
		SET d_ytd = $1
		WHERE d_w_id >= $2 AND d_w_id < $3`,
		)

		// Starting the background goroutine which will reset the w_ytd values periodically in warehouseWytdResetPeriod
		go p.startResetValueWorker()
	}

	if err := p.sr.Init(ctx, "payment", mcp); err != nil {
		return nil, err
	}

	return p, nil
}

func (p *payment) startResetValueWorker() {
	p.config.resetTableGrp.GoCtx(func(ctx context.Context) error {
		ticker := time.NewTicker(warehouseWytdResetPeriod)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:

				// Creating batches of maxRowsToUpdateTxn to avoid long-running txns
				for startRange := 0; startRange < p.config.warehouses; startRange += maxRowsToUpdateTxn {
					endRange := min(p.config.warehouses, startRange+maxRowsToUpdateTxn)

					if _, err := p.config.executeTx(
						ctx, p.mcp.Get(),
						func(tx pgx.Tx) error {
							if _, err := p.resetWarehouse.ExecTx(
								ctx, tx, wYtd, startRange, endRange,
							); err != nil {
								return errors.Wrap(err, "reset warehouse failed")
							}

							if _, err := p.resetDistrict.ExecTx(
								ctx, tx, ytd, startRange, endRange,
							); err != nil {
								return errors.Wrap(err, "reset district failed")
							}

							return nil
						}); err != nil {
						log.Errorf(ctx, "%v", err)
					}
				}
			}
		}
	})

}

func (p *payment) run(ctx context.Context, wID int) (interface{}, time.Duration, error) {
	p.config.auditor.paymentTransactions.Add(1)

	rng := rand.New(rand.NewPCG(uint64(timeutil.Now().UnixNano()), 1))

	d := paymentData{
		dID: rng.IntN(10) + 1,
		// hAmount is randomly selected within [1.00..5000.00]
		hAmount: float64(randInt(rng, 100, 500000)) / float64(100.0),
		hDate:   timeutil.Now(),
	}

	// 2.5.1.2: 85% chance of paying through home warehouse, otherwise
	// remote. This only applies to the customer update below, and not the
	// warehouse and district updates.
	// NOTE: If localWarehouses is set, keep all transactions local. This is for
	// testing only, as it violates the spec.
	if p.config.localWarehouses || rng.IntN(100) < 85 {
		d.cWID = wID
		d.cDID = d.dID
	} else {
		if len(p.config.multiRegionCfg.regions) > 0 {
			// For multi-region configurations, use the multi-region partitioner
			d.cWID = p.config.wMRPart.randActive(rng)
			// Find a cWID != w_id if there's more than 1 configured warehouse.
			for d.cWID == wID && p.config.activeWarehouses > 1 {
				d.cWID = p.config.wMRPart.randActive(rng)
			}
		} else {
			d.cWID = p.config.wPart.randActive(rng)
			// Find a cWID != w_id if there's more than 1 configured warehouse.
			for d.cWID == wID && p.config.activeWarehouses > 1 {
				d.cWID = p.config.wPart.randActive(rng)
			}
		}
		p.config.auditor.Lock()
		p.config.auditor.paymentRemoteWarehouseFreq[d.cWID]++
		p.config.auditor.Unlock()
		d.cDID = rng.IntN(10) + 1
	}

	// 2.5.1.2: The customer is randomly selected 60% of the time by last name
	// and 40% by number.
	if rng.IntN(100) < 60 {
		d.cLast = string(p.config.randCLast(rng, &p.a))
		p.config.auditor.paymentsByLastName.Add(1)
	} else {
		d.cID = p.config.randCustomerID(rng)
	}

	var onTxnStartDuration time.Duration
	var err error

	if p.config.literalImplementation {
		// Literal implementation without optimizations.
		onTxnStartDuration, err = p.config.executeTx(
			ctx, p.mcp.Get(),
			func(tx pgx.Tx) error {
				var wName, dName string

				// Select the warehouse data
				if err := p.literalSelectWarehouse.QueryRowTx(
					ctx, tx, wID,
				).Scan(&wName, &d.wStreet1, &d.wStreet2, &d.wCity, &d.wState, &d.wZip); err != nil {
					return errors.Wrap(err, "select warehouse failed")
				}

				// Update warehouse with payment
				if _, err := p.literalUpdateWarehouse.ExecTx(
					ctx, tx, d.hAmount, wID,
				); err != nil {
					return errors.Wrap(err, "update warehouse failed")
				}

				// Select district data before updating it below.
				if err := p.literalSelectDistrict.QueryRowTx(
					ctx, tx, wID, d.dID,
				).Scan(&dName, &d.dStreet1, &d.dStreet2, &d.dCity, &d.dState, &d.dZip); err != nil {
					return errors.Wrap(err, "select district failed")
				}

				// Update district with payment.
				if _, err := p.literalUpdateDistrict.ExecTx(
					ctx, tx, d.hAmount, wID, d.dID,
				); err != nil {
					return errors.Wrap(err, "update district failed")
				}

				// If we are selecting by last name, first find the relevant customer id and
				// then proceed.
				if d.cID == 0 {
					// 2.5.2.2 Case 2: Pick the middle row, rounded up, from the selection by last name.
					// The number of customers returned will typically be in the 1-5 range. Size the
					// slice for 10 customers to be safe.
					customers := make([]paymentData, 0, 10)
					if err := func() error {
						rows, err := p.literalSelectByLastName.QueryTx(ctx, tx, d.cWID, d.cDID, d.cLast)
						if err != nil {
							return err
						}
						defer rows.Close()

						for rows.Next() {
							// This data must all be returned by the transaction. See 2.5.3.4.
							// Unfortunately, this code isn't pretty to look at, but appears to be
							// necessary. The TPC-C search by last name requires us to query the
							// table and return all customers with the same last name, then pick
							// the middle row (rounded up), in the sorted result. To do this with
							// only one query, we have to return all the data for each customer,
							// store them all in memory, determine the correct row, and then store
							// that row for use later. We can't do this without caching all the
							// rows in memory because pgx4 doesn't allow us to scan rows multiple
							// times with the same query.
							var cFirst, cMiddle, cLast, cStreet1, cStreet2, cCity, cState, cZip, cPhone, cCredit string
							var cID int
							var cCreditLim, cDiscount, cBalance float64
							var cSince time.Time

							if err := rows.Scan(&cFirst, &cMiddle, &cLast, &cID, &cStreet1, &cStreet2,
								&cCity, &cState, &cZip, &cPhone, &cSince, &cCredit,
								&cCreditLim, &cDiscount, &cBalance); err != nil {
								return err
							}

							customers = append(customers,
								paymentData{
									cFirst:     cFirst,
									cMiddle:    cMiddle,
									cLast:      cLast,
									cStreet1:   cStreet1,
									cStreet2:   cStreet2,
									cCity:      cCity,
									cState:     cState,
									cZip:       cZip,
									cPhone:     cPhone,
									cCredit:    cCredit,
									cID:        cID,
									cCreditLim: cCreditLim,
									cDiscount:  cDiscount,
									cBalance:   cBalance,
									cSince:     cSince,
								})
						}
						return rows.Err()
					}(); err != nil {
						return errors.Wrap(err, "select customer failed")
					}
					cIdx := (len(customers) - 1) / 2
					d.cFirst = customers[cIdx].cFirst
					d.cMiddle = customers[cIdx].cMiddle
					d.cLast = customers[cIdx].cLast
					d.cID = customers[cIdx].cID
					d.cStreet1 = customers[cIdx].cStreet1
					d.cStreet2 = customers[cIdx].cStreet2
					d.cCity = customers[cIdx].cCity
					d.cState = customers[cIdx].cState
					d.cZip = customers[cIdx].cZip
					d.cPhone = customers[cIdx].cPhone
					d.cCredit = customers[cIdx].cCredit
					d.cCreditLim = customers[cIdx].cCreditLim
					d.cDiscount = customers[cIdx].cDiscount
					d.cBalance = customers[cIdx].cBalance
					d.cSince = customers[cIdx].cSince
				}

				// Update the customer with payment based on credit status
				if d.cCredit == "BC" {
					// In the bad credit case, the literal implementation should really retrieve the
					// c_data field, manipulate it on the application side, and then update it.
					// Doing that, however, would require extra logic here and likely wouldn't
					// change the efficiency much since it's the same number of round-trips to the
					// database. Instead, we retrieve the c_data field here but then update it using
					// the optimized update statement below (where the c_data update is prepared and
					// performed wholly in the SQL query). As a result, the retrieved c_data field
					// is scanned but not used.
					// FUTURE READER: please don't remove this select in an attempt to optimize this
					// code.
					if err := p.literalSelectDataForCustomerBad.QueryRowTx(
						ctx, tx, d.cWID, d.cDID, d.cID,
					).Scan(&d.cData); err != nil {
						return errors.Wrap(err, "selecting for customer with bad credit failed")
					}

					// Bad credit - need to update c_data and return it
					if _, err := p.literalUpdateCustomerBad.ExecTx(
						ctx, tx, d.hAmount, d.cWID, d.cDID, d.cID, d.dID, wID,
					); err != nil {
						return errors.Wrap(err, "update customer with bad credit failed")
					}
				} else {
					// Good credit - simpler update
					if _, err := p.literalUpdateCustomer.ExecTx(
						ctx, tx, d.hAmount, d.cWID, d.cDID, d.cID,
					); err != nil {
						return errors.Wrap(err, "update customer with good credit failed")
					}
					d.cData = "" // No data to return for good credit customers
				}

				// Update balance after the update
				d.cBalance -= d.hAmount

				hData := fmt.Sprintf("%s    %s", wName, dName)

				// Insert the history row.
				if _, err := p.insertHistory.ExecTx(
					ctx, tx,
					d.cID, d.cDID, d.cWID, d.dID, wID, d.hAmount, d.hDate.Format("2006-01-02 15:04:05"), hData,
				); err != nil {
					return errors.Wrap(err, "insert history failed")
				}

				return nil
			})
	} else {
		// Optimized implementation (original)
		onTxnStartDuration, err = p.config.executeTx(
			ctx, p.mcp.Get(),
			func(tx pgx.Tx) error {
				var wName, dName string

				// Update warehouse with payment
				if err := p.updateWarehouse.QueryRowTx(
					ctx, tx, d.hAmount, wID,
				).Scan(&wName, &d.wStreet1, &d.wStreet2, &d.wCity, &d.wState, &d.wZip); err != nil {
					return errors.Wrap(err, "update warehouse failed")
				}

				// Update district with payment
				if err := p.updateDistrict.QueryRowTx(
					ctx, tx, d.hAmount, wID, d.dID,
				).Scan(&dName, &d.dStreet1, &d.dStreet2, &d.dCity, &d.dState, &d.dZip); err != nil {
					return errors.Wrap(err, "update district failed")
				}

				// If we are selecting by last name, first find the relevant customer id and
				// then proceed.
				if d.cID == 0 {
					// 2.5.2.2 Case 2: Pick the middle row, rounded up, from the selection by last name.
					customers := make([]int, 0, 1)
					if err := func() error {
						rows, err := p.selectByLastName.QueryTx(ctx, tx, wID, d.dID, d.cLast)
						if err != nil {
							return err
						}
						defer rows.Close()
						for rows.Next() {
							var cID int
							if err := rows.Scan(&cID); err != nil {
								return err
							}
							customers = append(customers, cID)
						}
						return rows.Err()
					}(); err != nil {
						return errors.Wrap(err, "select customer failed")
					}
					cIdx := (len(customers) - 1) / 2
					d.cID = customers[cIdx]
				}

				// Update customer with payment.
				// If the customer has bad credit, update the customer's C_DATA and return
				// the first 200 characters of it, which is supposed to get displayed by
				// the terminal. See 2.5.3.3 and 2.5.2.2.
				if err := p.updateWithPayment.QueryRowTx(
					ctx, tx, d.hAmount, d.cWID, d.cDID, d.cID, d.dID, wID,
				).Scan(&d.cFirst, &d.cMiddle, &d.cLast, &d.cStreet1, &d.cStreet2,
					&d.cCity, &d.cState, &d.cZip, &d.cPhone, &d.cSince, &d.cCredit,
					&d.cCreditLim, &d.cDiscount, &d.cBalance, &d.cData,
				); err != nil {
					return errors.Wrap(err, "update customer failed")
				}

				hData := fmt.Sprintf("%s    %s", wName, dName)

				// Insert history line.
				if _, err := p.insertHistory.ExecTx(
					ctx, tx,
					d.cID, d.cDID, d.cWID, d.dID, wID, d.hAmount, d.hDate.Format("2006-01-02 15:04:05"), hData,
				); err != nil {
					return errors.Wrap(err, "insert history failed")
				}

				return nil
			})
	}
	if err != nil {
		return nil, 0, err
	}
	return d, onTxnStartDuration, nil
}
