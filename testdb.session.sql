-- Create the nps_withdrawal table
CREATE TABLE IF NOT EXISTS nps_withdrawal (
    id SERIAL PRIMARY KEY,
    pran_number VARCHAR(20) NOT NULL,
    member_id INTEGER NOT NULL,
    withdrawal_date DATE NOT NULL,
    amount DECIMAL(15, 2) NOT NULL,
    current_status VARCHAR(20) NOT NULL,
    type VARCHAR(20) NOT NULL
);
-- Seed NPS withdrawal data
INSERT INTO nps_withdrawal (
        pran_number,
        member_id,
        withdrawal_date,
        amount,
        current_status,
        type
    )
VALUES (
        'PRAN001',
        1,
        '2023-01-01',
        100000,
        'Approved',
        'Partial'
    ),
    (
        'PRAN002',
        2,
        '2023-02-15',
        150000,
        'Pending',
        'Full'
    ),
    (
        'PRAN003',
        3,
        '2023-03-30',
        200000,
        'Rejected',
        'Partial'
    ),
    (
        'PRAN004',
        4,
        '2023-05-10',
        250000,
        'Approved',
        'Full'
    ),
    (
        'PRAN005',
        5,
        '2023-06-25',
        300000,
        'Pending',
        'Partial'
    );