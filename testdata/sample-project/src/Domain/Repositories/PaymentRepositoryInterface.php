<?php
namespace App\Domain\Repositories;
use App\Domain\Entities\Payment;
interface PaymentRepositoryInterface { public function save(Payment $p): void; public function findById(string $id): ?Payment; }
