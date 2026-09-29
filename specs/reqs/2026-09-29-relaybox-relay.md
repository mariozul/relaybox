# 2026-09-29 Relaybox ReqSpec
Canonical source: user inline TRD; repository TRD is condensed and does not override it.
HIGH blast radius; schema/API/external boundaries yes; financial balances no. ReqSpec REQUIRED.

FR-ING-01 / AC-07: Given an isolated authenticated tenant and controlled dependencies, when the operation is exercised, then authenticated ingestion rejects tenant spoofing and malformed json.

FR-ING-02 / AC-02: Given an isolated authenticated tenant and controlled dependencies, when the operation is exercised, then rollback leaves neither event nor deliveries; committed work survives process restart.

FR-ING-03 / AC-01: Given an isolated authenticated tenant and controlled dependencies, when the operation is exercised, then concurrent duplicate ingestion returns one original id and one fanout set.

FR-SUB-01 / AC-08: Given an isolated authenticated tenant and controlled dependencies, when the operation is exercised, then create list and delete subscriptions using authenticated owner.

FR-SUB-02 / AC-04: Given an isolated authenticated tenant and controlled dependencies, when the operation is exercised, then foreign tenant cannot list or delete another owner subscription.

FR-DEL-01 / AC-09: Given an isolated authenticated tenant and controlled dependencies, when the operation is exercised, then only matching tenant/type receives post with deadline and traceparent.

FR-DEL-02 / AC-03: Given an isolated authenticated tenant and controlled dependencies, when the operation is exercised, then 500 then 200 retries only when due; 400 terminates at configured n; bodies close.

FR-DEL-03 / AC-05: Given an isolated authenticated tenant and controlled dependencies, when the operation is exercised, then bounded workers stop on cancellation during concurrent ingestion and delivery.

FR-DEL-04 / AC-10: Given an isolated authenticated tenant and controlled dependencies, when the operation is exercised, then retry and lease recovery reuse delivery id including crash after remote success.

FR-OBS-01 / AC-06: Given an isolated authenticated tenant and controlled dependencies, when the operation is exercised, then live process stays live; db outage yields readiness 503 within timeout.

FR-OBS-02 / AC-11: Given an isolated authenticated tenant and controlled dependencies, when the operation is exercised, then json logs carry trace/span; metric labels are finite and exclude secrets.

All inherited invariants: RULE-ARCH-01, RULE-ARCH-02, RULE-ARCH-03, RULE-DATA-01, RULE-DATA-02, RULE-DATA-03, RULE-RES-01, RULE-RES-02, RULE-RES-03, RULE-SEC-01, RULE-SEC-02, RULE-EVT-01, RULE-EVT-02, RULE-OBS-01, RULE-OBS-02, RULE-OBS-03, RULE-OBS-04
