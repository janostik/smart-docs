CREATE TABLE IF NOT EXISTS migrations (id INTEGER PRIMARY KEY AUTOINCREMENT,
                                                              name TEXT NOT NULL,
                                                                        applied_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP);