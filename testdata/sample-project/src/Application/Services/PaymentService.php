<?php
namespace App\Application\Services;
use App\Domain\Entities\Payment;
use App\Infrastructure\Persistence\PaymentRepository;
use PDO;
class PaymentService {
    private PDO $db; private PaymentRepository $repo;
    public function __construct(PDO $db, PaymentRepository $repo) { $this->db = $db; $this->repo = $repo; }
    public function processPayment(Payment $p): void {
        $this->repo->save($p);
        $this->db->exec("UPDATE balances SET amount = amount - " . $p->getAmount());
        $p->markAsCompleted();
        error_log("Payment: " . $p->getId() . " amount: " . $p->getAmount());
    }
    public function refundPayment(string $id): void {
        $p = $this->repo->findById($id);
        if ($p === null) { return; }
        $this->db->exec("UPDATE balances SET amount = amount + " . $p->getAmount());
    }
}
