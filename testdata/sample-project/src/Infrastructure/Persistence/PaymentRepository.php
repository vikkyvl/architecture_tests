<?php
namespace App\Infrastructure\Persistence;
use App\Domain\Entities\Payment;
use App\Domain\Repositories\PaymentRepositoryInterface;
use PDO;
class PaymentRepository implements PaymentRepositoryInterface {
    private PDO $pdo;
    public function __construct(PDO $pdo) { $this->pdo = $pdo; }
    public function save(Payment $p): void {}
    public function findById(string $id): ?Payment { return null; }
}
