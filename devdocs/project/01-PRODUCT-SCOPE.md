# Product scope and boundaries
Status: core product direction approved; expansion still requires explicit approval. Reviewed 22 September 2026. No application implementation is included.

## Planning assumptions, not settled business decisions
Nigeria, NGN, adult individual customers and a partner-backed operating model are the initial product-planning baseline. Confirm the legal entity, eligibility, custody/account structure, territories and provider agreements before enabling live services. Merely storing a currency code does not implement foreign exchange. Minors, joint accounts and non-residents require a separate eligibility and consent design.

## Launch capabilities
| Domain | Required scope | Acceptance boundary |
|---|---|---|
| Identity | Registration, contact verification, login, recovery, device/session management, MFA and transaction authorisation | Authentication and recovery tested independently; a transaction PIN alone is not login |
| KYC | Consent/lawful-basis records, partner identity verification, document/liveness flow where required, tier decisions, expiry and manual review | Current partner/regulatory policy approved; no hardcoded anonymous-money tier |
| Wallets | Account identity, ledger, available/held balances, limits, fees, immutable transaction history | Concurrent debits cannot overspend; customer liabilities reconcile to safeguarded/partner funds |
| Funding | Partner bank/virtual account instructions, incoming payment matching, duplicate-safe credit, unmatched deposits and returns | Credit only supported verified funding; missing webhook recovery exists |
| Transfers | Bank directory, account-name enquiry, internal transfers, bank payouts, beneficiaries, binding fee quote, receipts | Timeout recovery, delayed success, returned funds and reconciliation tested |
| Bills | Airtime, data, electricity, television, internet and approved additional billers; validation, quotation, vend, status and token recovery | Financial settlement and service fulfilment are separately tracked |
| Customer experience | Searchable history, statements, notifications, saved billers, support, disputes and account controls | Mobile and web recover pending transactions after restart/refresh |
| Operations | Staff roles, maker/checker, KYC/risk queues, support cases, provider status, limits, pricing, audit | No direct balance editing, no self-approval and no privileged secrets in client code |
| Finance | Settlement, three-way reconciliation, suspense, exception ageing, float and fee reporting | Bank/provider/ledger differences have evidence, ownership and escalation |
| Platform | Safe APIs, observability, private document storage, backups, restore, CI, runbooks and compatibility | Readiness demonstrated by executed tests and release evidence |

## Growth catalogue — proposals, not automatic approval
Scheduled payments; explicit recurring mandates; bill reminders; saved payment templates; household budgets; spending categories; savings pots without yield; requests for payment; split expenses; QR payments; payment links; invoices; merchant collections; subscriptions; business KYB; company roles and approvals; bulk disbursements; payroll; batch beneficiary validation; employee expense controls; loyalty points; cashback; referrals; promotions; partner APIs and outbound webhooks; accounting exports; donations with beneficiary governance; account-data connections through authorised providers.

Each growth epic must be decomposed into API, mobile, web, admin, finance, support, security and release acceptance tasks before implementation. A reminder is not a direct-debit mandate. A savings pot is not an interest-bearing deposit. A campaign screen does not implement donation custody or beneficiary governance.

## Separately gated product families
Physical/virtual card issuance; credit, overdrafts and BNPL; yield-bearing savings; investments; insurance; foreign exchange and remittances; agent banking and POS; escrow; joint/group wallets; minors' products; digital assets. These require their own commercial, legal, partner, fraud, accounting and dispute decisions. Keep unavailable products disabled and do not advertise them as launched.

## Commercial design
Assess contribution margin per transaction: approved customer fee plus earned partner commission less provider fees, identity checks, notifications, infrastructure allocation, fraud/loss allowance, support and applicable approved taxes/charges. Do not assume fixed provider prices or universal bill-payment commissions. Treasury float and customer money are not revenue. Price changes require effective dates, review, customer disclosure and immutable quote snapshots.

## Out of scope for this planning delivery
No selected licences, provider contract, production hostname, live transaction, application code, runtime screenshot, release or deployment. The programme is broad but intentionally not claimed to enumerate every future financial product.
