-- Database initialization script for roachprod-centralized
-- This script sets up the initial database schema and user

-- Create the roachprod database if it doesn't exist
CREATE DATABASE IF NOT EXISTS roachprod;

-- Use the roachprod database
USE roachprod;

-- Create a dedicated user for the application (in production)
-- Note: This is commented out for local development with insecure mode
-- CREATE USER IF NOT EXISTS roachprod_user WITH PASSWORD 'secure_password';
-- GRANT ALL ON DATABASE roachprod TO roachprod_user;

-- The tables will be created automatically by the application's migration system
-- This script is mainly for database and user setup