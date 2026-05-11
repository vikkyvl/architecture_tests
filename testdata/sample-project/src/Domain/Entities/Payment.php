<?php
namespace App\Domain\Entities;
class Payment {
    private string $id; private float $amount; private string $status;
    public function __construct(string $id, float $amount) { $this->id = $id; $this->amount = $amount; $this->status = 'pending'; }
    public function getId(): string { return $this->id; }
    public function getAmount(): float { return $this->amount; }
    public function markAsCompleted(): void { $this->status = 'completed'; }
}
